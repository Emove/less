package channel

import (
	"context"
	"errors"
	"github.com/emove/less"
	"github.com/emove/less/log"
	"github.com/emove/less/pkg/io"
	"github.com/emove/less/transport"
	"net"
	"sync/atomic"
)

const (
	inactive = iota
	readable
	writeable
	readWriteMode
)

var (
	ErrChannelClosed       = errors.New("channel has been closed")
	ErrChannelReaderClosed = errors.New("channel reader has been closed")
	ErrChannelWriterClosed = errors.New("channel writer has been closed")
)

var _ less.Channel = (*Channel)(nil)

type Channel struct {
	ctx       context.Context
	conn      transport.Connection
	state     int32
	done      chan struct{}
	pl        *pipeline
	side      int // represents client's channel or server's channel
	lastRead  int64
	lastWrite int64
}

func NewChannel(con transport.Connection, factory PipelineFactory) *Channel {
	ch := &Channel{
		ctx:   context.Background(),
		pl:    factory(),
		conn:  con,
		state: inactive,
		done:  make(chan struct{}),
	}
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
		return ch.pl.FireOutbound(ch, msg)
	}
	return ErrChannelWriterClosed
}

func (ch *Channel) IsActive() bool {
	return atomic.LoadInt32(&ch.state)&readWriteMode != 0 && ch.conn.IsActive()
}

func (ch *Channel) CloseReader() {
	ch.close(readable)
}

func (ch *Channel) CloseWriter() {
	ch.close(writeable)
}

func (ch *Channel) Readable() bool {
	return ch.calState(readable)
}

func (ch *Channel) Writeable() bool {
	return ch.calState(writeable)
}

func (ch *Channel) Close(ctx context.Context, err error) error {

	old := atomic.LoadInt32(&ch.state)
	if inactive == old || !atomic.CompareAndSwapInt32(&ch.state, old, inactive) {
		return ErrChannelClosed
	}
	ch.close(inactive)

	return nil
}

// AddOnChannelClosed adds OnChannelClosed for channel
func (ch *Channel) AddOnChannelClosed(onChannelClosed ...less.OnChannelClosed) {
	ch.pl.AddOnChannelClosed(onChannelClosed...)
}

// AddInboundMiddleware adds inbound middleware for current channel only
func (ch *Channel) AddInboundMiddleware(mw ...less.Middleware) {
	ch.pl.AddInbound(mw...)
}

// AddOutboundMiddleware adds outbound middleware for current channel only
func (ch *Channel) AddOutboundMiddleware(mw ...less.Middleware) {
	ch.pl.AddOutbound(mw...)
}

// ====================================== implements stater ============================================ //

func (ch *Channel) Channel() *Channel {
	return ch
}

func (ch *Channel) LastRead() int64 {
	return atomic.LoadInt64(&ch.lastRead)
}

func (ch *Channel) LastWrite() int64 {
	return atomic.LoadInt64(&ch.lastWrite)
}

// ====================================== internal functions ============================================ //

func (ch *Channel) Reader() (io.Reader, error) {
	if !ch.calState(readable) {
		return nil, ErrChannelReaderClosed
	}
	return ch.conn.Reader(), nil
}

func (ch *Channel) Writer() (io.Writer, error) {
	if !ch.calState(readable) {
		return nil, ErrChannelWriterClosed
	}
	return ch.conn.Writer(), nil
}

func (ch *Channel) SetContext(ctx context.Context) {
	ch.ctx = ctx
}

func (ch *Channel) Activate(ctx context.Context) error {
	err := ch.pl.FireOnChannel(ch, ctx)
	if err == nil {
		atomic.StoreInt32(&ch.state, readWriteMode)
	}
	log.Infof("new channel active from: %s", ch.conn.RemoteAddr().String())
	return err
}

func (ch *Channel) TriggerInbound(msg interface{}) error {
	return ch.pl.FireInbound(ch, msg)
}

func (ch *Channel) Side() int {
	return ch.side
}

func (ch *Channel) close(state int32) {
	for {
		old := atomic.LoadInt32(&ch.state)
		if old&state == state {
			if atomic.CompareAndSwapInt32(&ch.state, old, old^state) {
				return
			}
		} else {
			return
		}
	}
}

func (ch *Channel) calState(state int32) bool {
	return atomic.LoadInt32(&ch.state)&state == state
}
