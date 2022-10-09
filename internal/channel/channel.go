package channel

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/pkg/io"
	_go "github.com/emove/less/pkg/pool/go"
	"github.com/emove/less/transport"
)

const (
	inactive = iota
	readable
	writeable
	readWriteMode
)

const (
	Client = 1
	Server = 2
)

var (
	ErrChannelClosed       = errors.New("channel already closed")
	ErrChannelReaderClosed = errors.New("channel reader was closed")
	ErrChannelWriterClosed = errors.New("channel writer was closed")
)

type Channel struct {
	ctx   context.Context
	conn  transport.Connection
	state int32
	done  chan struct{}
	pl    *pipeline
	tasks *WaitGroup

	// represents client's channel or server's channel
	side int

	lastRead  int64
	lastWrite int64
	// guard the following
	mu sync.Mutex
	// records channel idle time
	idle time.Time
}

func NewChannel(con transport.Connection, side int, factory PipelineFactory) *Channel {
	ch := &Channel{
		ctx:   context.Background(),
		conn:  con,
		state: inactive,
		done:  make(chan struct{}),
		side:  side,
		tasks: NewWaitGroup(),
		idle:  time.Now(),
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
	if inactive == old || !atomic.CompareAndSwapInt32(&ch.state, old, inactive) {
		return ErrChannelClosed
	}
	ch.close(inactive)

	// execute in a goroutine to avoid tasks WaitGroup deadlock
	_go.Submit(func() {
		defer func() {
			close(ch.done)
			// reuse pipeline
			ch.pl.Release()
			// close connection
			_ = ch.conn.Close()
		}()

		//log.Debugf("[channel] waiting for read tasks")
		ch.tasks.WaitReadTask()
		//log.Debugf("[channel] read tasks done")

		// fire OnChannelClosed hook after inbound tasks finished
		// to avoid causing errors in case of customer holding that
		// something like session about channel
		ch.pl.FireOnChannelClosed(err)

		done := make(chan struct{})
		_go.Submit(func() {
			// waiting for all outbound tasks done
			//log.Debugf("[channel] waiting for write tasks")
			ch.tasks.WaitWriteTask()
			//log.Debugf("[channel] write tasks done")
			close(done)
		})

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}

	})

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

func (ch *Channel) GetChannel() *Channel {
	return ch
}

func (ch *Channel) GetIdleTime() time.Time {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	return ch.idle
}

func (ch *Channel) GetLastRead() int64 {
	return atomic.LoadInt64(&ch.lastRead)
}

func (ch *Channel) GetLastWrite() int64 {
	return atomic.LoadInt64(&ch.lastWrite)
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

func (ch *Channel) Readable() bool {
	return ch.calState(readable)
}

func (ch *Channel) Writeable() bool {
	return ch.calState(writeable)
}

func (ch *Channel) WriteDirectly(msg interface{}) error {
	if ch.calState(writeable) {
		return ch.pl.FireOutbound(msg)
	}
	return ErrChannelWriterClosed
}

func (ch *Channel) Side() int {
	return ch.side
}

// Recorder returns a middleware to record channel state
func Recorder(event int) less.Middleware {
	return func(handler less.Handler) less.Handler {
		return func(ctx context.Context, c less.Channel, message interface{}) error {
			ch := c.(*Channel)
			ch.addTask(event)
			err := handler(ctx, ch, message)
			ch.tasks.Done(event)
			return err
		}
	}
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

func (ch *Channel) addTask(event int) {
	ch.tasks.Add(event)

	switch event {
	case ReadEvent:
		atomic.StoreInt64(&ch.lastRead, time.Now().UnixNano())
	case WriteEvent:
		atomic.StoreInt64(&ch.lastWrite, time.Now().UnixNano())
	}

	// indicates channel is busy
	if ch.idle.IsZero() {
		return
	}

	ch.mu.Lock()
	defer ch.mu.Unlock()

	// check again
	if ch.idle.IsZero() {
		return
	}
	ch.idle = time.Time{}

	_go.Submit(func() {
		fin := make(chan struct{})
		_go.Submit(func() {
			ch.tasks.Wait()
			close(fin)
		})

		for {
			select {
			case <-ch.done:
				return
			case <-fin:
				func() {
					ch.mu.Lock()
					defer ch.mu.Unlock()
					ch.idle = time.Now()
				}()
				return
			}
		}
	})
}
