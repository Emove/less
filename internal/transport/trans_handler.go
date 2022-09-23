package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/emove/less"
	"github.com/emove/less/internal/channel"
	less_atomic "github.com/emove/less/internal/utils/atomic"
	"github.com/emove/less/internal/utils/recovery"
	"github.com/emove/less/log"
	"github.com/emove/less/pkg/io"
	_go "github.com/emove/less/pkg/pool/go"
	"github.com/emove/less/pkg/router"
	"github.com/emove/less/transport"
)

type BoundHandler interface {
	OnRead(ch *channel.Channel, reader io.Reader) (err error)
	OnWrite(ch *channel.Channel, writer io.Writer, msg interface{}) error
}

type ctxChannelKey struct{}

type TransHandler interface {
	transport.EventDriver
	BoundHandler
	GracefulCloser
}

func NewTransHandler(ops ...Option) TransHandler {
	opts := defaultTransOptions
	for _, op := range ops {
		op(opts)
	}
	th := &transHandler{
		ops:          opts,
		channels:     sync.Map{},
		channelCount: less_atomic.AtomicInt64(0),
	}
	th.pipelineFactory = channel.NewPipelineFactory(opts.onChannel, opts.onChannelClosed, opts.inbound, opts.outbound, newRouter(opts.router), th.outboundHandler)
	return th
}

var _ TransHandler = (*transHandler)(nil)

const (
	serving = iota
	closed
)

type transHandler struct {
	state int32
	ops   *Options

	channels     sync.Map
	channelCount less_atomic.AtomicInt64

	pipelineFactory channel.PipelineFactory
	closingCtx      context.Context
}

func (th *transHandler) OnConnect(ctx context.Context, con transport.Connection) (c context.Context, err error) {

	if !th.isActive() {
		return ctx, errors.New("connect request was refused")
	}
	var ch *channel.Channel
	defer func() {
		recovery.Recover(func(e error) {
			log.Errorw("err", fmt.Sprintf("panic on channel: %v", e))
			err = e
		})
		closingCtx := context.Background()
		if !th.isActive() {
			err = errors.New("transport was closed")
			closingCtx = th.closingCtx
		}
		if err != nil && ch != nil {
			_ = ch.Close(closingCtx, err)
		}
	}()

	log.Debugf("receive a connect request from: %s", con.RemoteAddr().String())

	// check connection limit
	if th.ops.maxConnectionSize > 0 && th.channelCount.Value() > int64(th.ops.maxConnectionSize) {
		log.Infof("new connect request was refused, concurrent channel nums: %d", th.channelCount.Value())
		return ctx, errors.New("connection number out of limit")
	}

	ch = channel.NewChannel(con, th.pipelineFactory)

	if err = ch.GetPipeline().FireOnChannel(ctx); err != nil {
		log.Debugf("connect request from: %s failed, err: %v", con.RemoteAddr().String(), err)
		return ctx, err
	}

	th.channelCount.Inc()
	th.channels.Store(ch, struct{}{})

	return context.WithValue(ctx, ctxChannelKey{}, ch), nil
}

func (th *transHandler) OnMessage(ctx context.Context, _ transport.Connection) error {

	if !th.isActive() {
		return errors.New("request was refused")
	}

	ch := ctx.Value(ctxChannelKey{}).(*channel.Channel)

	reader, err := ch.Reader()
	if err != nil {
		return nil
	}

	defer reader.Release()
	return th.OnRead(ch, reader)
}

func (th *transHandler) OnConnClosed(ctx context.Context, _ transport.Connection, err error) {
	ch := ctx.Value(ctxChannelKey{}).(*channel.Channel)

	th.closeChannel(ctx, ch, err)
}

func (th *transHandler) OnRead(ch *channel.Channel, reader io.Reader) error {

	if !th.isActive() {
		return errors.New("transport was closed")
	}

	defer recovery.Recover(func(err error) {
		th.closeChannel(context.Background(), ch, err)
	})

	// do decode
	msg, err := th.ops.packetCodec.Decode(reader, th.ops.payloadCodec)
	if err != nil {
		// close channel
		th.closeChannel(context.Background(), ch, err)
		return err
	}

	if th.ops.maxReceiveMessageSize > 0 && uint32(reader.Length()) > th.ops.maxReceiveMessageSize {
		log.Errorf("receive a message but message size greater than max-receive-message-size, message size: %d, max: %d", reader.Length(), th.ops.maxReceiveMessageSize)
	}

	_go.Submit(func() {
		if err = ch.GetPipeline().FireInbound(msg); err != nil {
			log.Errorw("remote", ch.RemoteAddr(), log.DefaultMsgKey, msg, "err", err)
		}
	})

	return nil
}

func (th *transHandler) OnWrite(ch *channel.Channel, writer io.Writer, msg interface{}) error {
	defer recovery.Recover(func(err error) {
		th.closeChannel(context.Background(), ch, err)
	})

	if serving != atomic.LoadInt32(&th.state) {
		return fmt.Errorf("transport has been closed")
	}

	if th.ops.maxSendMessageSize > 0 {
		// TODO limit writer buffer
	}

	// do encode
	return th.ops.packetCodec.Encode(msg, writer, th.ops.payloadCodec)
}

func (th *transHandler) Close(ctx context.Context, err error) error {

	if !atomic.CompareAndSwapInt32(&th.state, serving, closed) {
		return nil
	}
	th.closingCtx = ctx

	done := make(chan struct{})
	closingChannels := sync.WaitGroup{}

	_go.Submit(func() {
		th.channels.Range(func(key, value interface{}) bool {
			ch := key.(*channel.Channel)
			closingChannels.Add(1)
			_go.Submit(func() {
				th.closeChannel(ctx, ch, err)
				closingChannels.Done()
			})
			return true
		})

		// wait for all tasks
		closingChannels.Wait()

		close(done)
	})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return nil
		}
	}
}

func (th *transHandler) closeChannel(ctx context.Context, ch *channel.Channel, err error) {
	if _, ok := th.channels.LoadAndDelete(ch); !ok {
		return
	}

	_ = ch.Close(ctx, err)
}

func (th *transHandler) isActive() bool {
	if serving != atomic.LoadInt32(&th.state) {
		return false
	}
	return true
}

func (th *transHandler) outboundHandler(_ context.Context, ch less.Channel, message interface{}) error {
	w := ch.(*channel.Channel).Writer()
	defer w.Release()
	return th.OnWrite(ch.(*channel.Channel), w, message)
}

func newRouter(router router.Router) less.Middleware {
	return func(handler less.Handler) less.Handler {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			//if err := handler(ctx, ch, message);err != nil {
			//	return err
			//}
			h, err := router(ctx, ch, message)
			if err != nil {
				return err
			}
			return h(ctx, ch, message)
		}
	}
}
