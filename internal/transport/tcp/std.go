//go:build windows
// +build windows

package tcp

import (
	"context"
	"net"
	"sync/atomic"
	"time"

	"less/internal/conn"
	"less/internal/io/reader"
	"less/internal/io/writer"
	"less/pkg/io"
)

// WrapConnection wrap net.Conn to transport.Connection
func WrapConnection(conn net.Conn) conn.Connection {
	ctx, cancelFunc := context.WithCancel(context.Background())
	return &connection{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		delegate:   conn,
	}
}

var _ conn.Connection = (*connection)(nil)

// connection implements transport.Connection
type connection struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
	delegate   net.Conn

	closed      int32
	readTimeout time.Duration
}

func (c *connection) Read(buf []byte) (n int, err error) {
	r := reader.NewBufferReaderWithBuf(c, buf)
	if c.readTimeout > 0 {
		r = reader.NewTimeoutReader(r, c.readTimeout)
	}
	size := len(buf)
	r = reader.NewLimitReader(r, uint32(size))
	_, err = r.Next(size)
	if err != nil {
		return -1, err
	}
	return size, nil
}

// Reader returns a reader
func (c *connection) Reader() io.Reader {
	r := reader.NewBufferReader(c)
	if c.readTimeout > 0 {
		r = reader.NewTimeoutReader(r, c.readTimeout)
	}
	return r
}

// Writer returns a writer
func (c *connection) Writer() io.Writer {
	return writer.NewBufferWriter(c)
}

func (c *connection) IsActive() bool {
	return atomic.LoadInt32(&c.closed) == conn.Active
}

// Close closes the net.Conn
func (c *connection) Close() error {
	if atomic.CompareAndSwapInt32(&c.closed, conn.Active, conn.Inactive) {
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

// SetReadTimeout sets the timeout for future Read calls wait
func (c *connection) SetReadTimeout(t time.Duration) error {
	if t >= 0 {
		c.readTimeout = t
	}
	return nil
}
