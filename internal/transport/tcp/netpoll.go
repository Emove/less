//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package tcp

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"less/internal/conn"
	"less/pkg/io"

	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/netpoll/mux"
)

func WrapConnection(con netpoll.Connection) conn.Connection {
	c := connPool.Get().(*connection)
	c.delegate = con
	return c
}

func (c *connection) Read(buf []byte) (n int, err error) {
	return c.delegate.Read(buf)
}

func (c *connection) Reader() io.Reader {
	c.r.delegate = c.delegate.Reader()
	return c.r
}

func (c *connection) Writer() io.Writer {
	c.w.delegate = c.delegate.Writer()
	return c.w
}

func (c *connection) IsActive() bool {
	return c.delegate.IsActive()
}

func (c *connection) Close() error {
	if atomic.CompareAndSwapUint32(&c.closed, conn.Active, conn.Inactive) {
		return c.delegate.Close()
	}
	return nil
}

func (c *connection) LocalAddr() net.Addr {
	return c.delegate.LocalAddr()
}

func (c *connection) RemoteAddr() net.Addr {
	return c.delegate.RemoteAddr()
}

func (c *connection) SetReadTimeout(t time.Duration) error {
	return c.delegate.SetReadTimeout(t)
}

func (w *writer) Write(buf []byte) (n int, err error) {
	return w.delegate.WriteBinary(buf)
}

func (w *writer) Malloc(n int) (buf []byte) {
	malloc, _ := w.delegate.Malloc(n)
	return malloc
}

func (w *writer) MallocLength() int {
	return w.delegate.MallocLen()
}

func (w *writer) Flush() error {
	return w.delegate.Flush()
}

func (w *writer) Release() {
	w.delegate = nil
}

func (r *reader) Next(n int) (buf []byte, err error) {
	return r.delegate.Next(n)
}

func (r *reader) Peek(n int) (buf []byte, err error) {
	return r.delegate.Peek(n)
}

func (r *reader) Skip(n int) (err error) {
	return r.delegate.Skip(n)
}

func (r *reader) Until(delim byte) (line []byte, err error) {
	return r.delegate.Until(delim)
}

func (r *reader) Release() {
	_ = r.delegate.Release()
}

var connPool = sync.Pool{
	New: func() interface{} {
		return &connection{
			r: &reader{},
			w: &writer{},
		}
	},
}

// connection implements conn.Connection
type connection struct {
	delegate netpoll.Connection
	sq       *mux.ShardQueue
	r        *reader
	w        *writer
	closed   uint32
}

var _ io.Reader = (*reader)(nil)

// reader implements io.Reader
type reader struct {
	delegate netpoll.Reader
}

var _ io.Writer = (*writer)(nil)

// writer implements io.Writer
type writer struct {
	delegate netpoll.Writer
}
