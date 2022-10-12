package tcp

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"github.com/emove/less/pkg/io"
	"github.com/emove/less/pkg/io/reader"
	"github.com/emove/less/pkg/io/writer"
	trans "github.com/emove/less/transport"
)

// WrapConnection wraps net.Conn to conn.Connection
func WrapConnection(conn net.Conn) trans.Connection {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &connection{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		delegate:   conn,
	}
}

var _ trans.Connection = (*connection)(nil)

// connection implements conn.Connection
type connection struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	delegate   net.Conn

	closed      int32
	readTimeout time.Duration
}

func (c *connection) Read(buf []byte) (n int, err error) {
	return c.delegate.Read(buf)
}

// Reader returns a reader
func (c *connection) Reader() io.Reader {
	r := reader.NewBufferReader(c)
	//if c.readTimeout > 0 {
	//	r = reader.NewTimeoutReader(r, c.readTimeout)
	//}
	return r
}

// Writer returns a writer
func (c *connection) Writer() io.Writer {
	return writer.NewBufferWriter(c.delegate)
}

// IsActive returns false when connection closed
func (c *connection) IsActive() bool {
	return atomic.LoadInt32(&c.closed) == trans.Active
}

// Close closes the net.Conn
func (c *connection) Close() error {
	if atomic.CompareAndSwapInt32(&c.closed, trans.Active, trans.Inactive) {
		return c.delegate.Close()
	}
	return nil
}

// LocalAddr returns the local address
func (c *connection) LocalAddr() net.Addr {
	return c.delegate.LocalAddr()
}

// RemoteAddr returns the remote address
func (c *connection) RemoteAddr() net.Addr {
	return c.delegate.RemoteAddr()
}
