package channel

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	"github.com/emove/less"
	"github.com/emove/less/pkg/io"
	"github.com/emove/less/transport/conn"
)

const (
	closed = iota
	readable
	writeable
	readWriteMode
)

var (
	ErrChannelClosed       = errors.New("channel was closed")
	ErrChannelReaderClosed = errors.New("channel reader was closed")
	ErrChannelWriterClosed = errors.New("channel writer was closed")
)

type Channel struct {
	ctx       context.Context
	conn      conn.Connection
	state     int32
	stateLock sync.Locker
	pl        *pipeline

	inboundTasks  sync.WaitGroup
	outboundTasks sync.WaitGroup
}

func NewChannel(con conn.Connection, factory PipelineFactory) *Channel {
	ch := &Channel{
		ctx:           context.Background(),
		conn:          con,
		state:         closed,
		stateLock:     &sync.Mutex{},
		inboundTasks:  sync.WaitGroup{},
		outboundTasks: sync.WaitGroup{},
	}
	ch.pl = factory(ch)
	return ch
}

// ====================================== implements less.Channel ============================================ //

func (ch *Channel) Context() context.Context {
	return ch.ctx
}

func (ch *Channel) RemoteAddr() net.Addr {
	return ch.conn.RemoteAddr()
}

func (ch *Channel) LocalAddr() net.Addr {
	return ch.conn.LocalAddr()
}

func (ch *Channel) Write(msg interface{}) error {
	if ch.calState(writeable) {
		ch.outboundTasks.Add(1)
		defer ch.outboundTasks.Done()
		return ch.pl.FireOutbound(msg)
	}
	return ErrChannelWriterClosed
}

func (ch *Channel) IsActive() bool {
	return ch.conn.IsActive()
}

func (ch *Channel) CloseReader() {
	ch.close(readable)
}

func (ch *Channel) CloseWriter() {
	ch.close(writeable)
}

func (ch *Channel) Close(ctx context.Context, err error) error {

	old := atomic.LoadInt32(&ch.state)
	if closed == old || !atomic.CompareAndSwapInt32(&ch.state, old, closed) {
		return ErrChannelClosed
	}

	defer func() {
		// reuse pipeline
		ch.pl.Release()
		// close connection
		_ = ch.conn.Close()
	}()

	ch.inboundTasks.Wait()
	ch.pl.FireOnChannelClosed(err)

	done := make(chan struct{})
	go func() {
		// waiting for all task done
		ch.outboundTasks.Wait()
		close(done)
	}()

	for {
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

}

func (ch *Channel) AddOnChannelClosed(onChannelClosed ...less.OnChannelClosed) {
	ch.pl.AddOnChannelClosed(onChannelClosed...)
}

func (ch *Channel) AddInboundMiddleware(mw ...less.Middleware) {
	ch.pl.AddInbound(mw...)
}

func (ch *Channel) AddOutboundMiddleware(mw ...less.Middleware) {
	ch.pl.AddOutbound(mw...)
}

// ====================================== internal functions ============================================ //

func (ch *Channel) Reader() (io.Reader, error) {
	if !ch.calState(readable) {
		return nil, ErrChannelReaderClosed
	}
	return ch.conn.Reader(), nil
}

func (ch *Channel) Writer() io.Writer {
	return ch.conn.Writer()
}

func (ch *Channel) SetContext(ctx context.Context) {
	ch.ctx = ctx
}

func (ch *Channel) GetPipeline() *pipeline {
	return ch.pl
}

func (ch *Channel) close(mod int32) {
	for {
		old := atomic.LoadInt32(&ch.state)
		if old&mod == mod {
			if atomic.CompareAndSwapInt32(&ch.state, old, old^mod) {
				return
			}
		} else {
			return
		}
	}
}

func (ch *Channel) active() {
	atomic.StoreInt32(&ch.state, readWriteMode)
}

func (ch *Channel) calState(state int32) bool {
	return atomic.LoadInt32(&ch.state)&state == state
}
