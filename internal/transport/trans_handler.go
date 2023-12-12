package transport

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/emove/less"
	less_atomic "github.com/emove/less/internal/atomic"
	"github.com/emove/less/internal/channel"
	"github.com/emove/less/internal/recovery"
	"github.com/emove/less/log"
	"github.com/emove/less/transport"
)

type BoundHandler interface {
	OnRead(ch *channel.Channel, msg interface{}) (err error)
	OnWrite(ch *channel.Channel, msg interface{}) error
}

type ctxChannelKey struct{}

type TransHandler interface {
	transport.EventDriver
	BoundHandler
	Close() error
}

func NewSrvTransHandler(ctx context.Context, ops ...Option) TransHandler {
	opts := defaultTransOptions
	for _, op := range ops {
		op(opts)
	}
	th := &svrTransHandler{
		ops:          opts,
		ctx:          ctx,
		channelCount: less_atomic.AtomicInt64(0),
	}
	onChannelClosed := append([]less.OnChannelClosed{func(ctx context.Context, ch less.Channel, err error) {
		th.closeChannel()
	}}, th.ops.onChannelClosed...)

	inbound := opts.inbound
	outbound := opts.outbound

	th.pipelineFactory = channel.NewPipelineFactory(opts.onChannel, onChannelClosed, inbound, outbound, NewRouterMiddleware(opts.router), OutboundHandler(th))

	log.Infow("max-channel-size", opts.maxChannelSize, "max-send-message-size", opts.maxSendMessageSize, "max-receive-message-size", opts.maxReceiveMessageSize)
	log.Infow("packet-codec", opts.packetCodec.Name(), "payload-codec", opts.payloadCodec.Name())

	return th
}

var _ TransHandler = (*svrTransHandler)(nil)

const (
	serving = iota
	closed
)

type svrTransHandler struct {
	state           int32
	ops             *options
	ctx             context.Context
	channelCount    less_atomic.AtomicInt64
	pipelineFactory channel.PipelineFactory
}

func (th *svrTransHandler) OnConnect(ctx context.Context, con transport.Connection) (c context.Context, err error) {

	if !th.isActive() {
		return ctx, errors.New("server has been shutdown")
	}
	var ch *channel.Channel
	defer func() {
		recovery.Recover(func(e error) {
			log.Errorw("err", fmt.Sprintf("panic on channel: %v", e))
			err = e
		})
		if err != nil && ch != nil {
			ch.Close(err)
		}
	}()

	log.Debugf("receive a connect request from: %s", con.RemoteAddr().String())

	// check connection limit
	if th.ops.maxChannelSize > 0 && th.channelCount.Inc() > int64(th.ops.maxChannelSize) {
		th.channelCount.Dec()
		log.Infof("new connection request was refused, current channel nums: %d", th.channelCount.Value())
		return ctx, errors.New("connection number out of limit")
	}

	ch = channel.NewChannel(con, th.pipelineFactory)

	if err = ch.Activate(ctx); err != nil {
		log.Debugf("connect request from: %s failed, err: %v", con.RemoteAddr().String(), err)
		return ctx, err
	}

	return context.WithValue(ctx, ctxChannelKey{}, ch), nil
}

func (th *svrTransHandler) OnMessage(ctx context.Context, _ transport.Connection) error {

	if !th.isActive() {
		return errors.New("request was refused")
	}

	ch := ctx.Value(ctxChannelKey{}).(*channel.Channel)

	reader, err := ch.Reader()
	if err != nil {
		return nil
	}

	defer reader.Release()
	if !th.isActive() {
		return errors.New("transport was closed")
	}

	defer recovery.Recover(func(err error) {
		ch.Close(err)
	})

	msg, err := th.ops.packetCodec.Decode(reader, th.ops.payloadCodec)
	if err != nil {
		ch.Close(err)
		return err
	}

	// TODO 改用limitWriter
	if th.ops.maxReceiveMessageSize > 0 && uint32(reader.Length()) > th.ops.maxReceiveMessageSize {
		log.Errorf("receive a message but message size greater than max-receive-message-size, message size: %d, max: %d", reader.Length(), th.ops.maxReceiveMessageSize)
		return nil
	}
	return th.OnRead(ch, msg)
}

func (th *svrTransHandler) OnConnClosed(ctx context.Context, _ transport.Connection, err error) {
	ch := ctx.Value(ctxChannelKey{}).(*channel.Channel)
	ch.Close(err)
}

func (th *svrTransHandler) OnRead(ch *channel.Channel, msg interface{}) error {
	var err error
	if err = ch.TriggerInbound(msg); err != nil {
		log.Errorw("remote", ch.RemoteAddr(), log.DefaultMsgKey, msg, "err", err)
	}
	return nil
}

func (th *svrTransHandler) OnWrite(ch *channel.Channel, msg interface{}) error {
	defer recovery.Recover(func(err error) {
		ch.Close(err)
	})

	if serving != atomic.LoadInt32(&th.state) {
		return fmt.Errorf("transport has been closed")
	}
	writer, err := ch.Writer()
	if err != nil {
		return err
	}
	writer.Release()
	if err := th.ops.packetCodec.Encode(msg, writer, th.ops.payloadCodec); err != nil {
		return err
	}
	writer.Flush()
	writer.Release()
	return nil
}

func (th *svrTransHandler) Close() error {
	if !atomic.CompareAndSwapInt32(&th.state, serving, closed) {
		return nil
	}
	return nil
}

func (th *svrTransHandler) closeChannel() {
	th.channelCount.Dec()
}

func (th *svrTransHandler) isActive() bool {
	return serving == atomic.LoadInt32(&th.state)
}
