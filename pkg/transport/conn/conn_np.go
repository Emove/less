//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package conn

import (
	"github.com/cloudwego/netpoll"
	"less/pkg/transport"
	"net"
	"sync"
	"time"
)

func WrapConnection(con netpoll.Connection) transport.Connection {
	c := connPool.Get().(*connection)
	c.conn = con
	return c
}

func (c *connection) Reader() transport.Reader {
	c.r.delegate = c.conn.Reader()
	return c.r
}

func (c *connection) Writer() transport.Writer {
	c.w.delegate = c.conn.Writer()
	return c.w
}

func (c *connection) Close() error {
	return c.conn.Close()
}

func (c *connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *connection) SetReadTimeout(t time.Duration) error {
	return c.conn.SetReadTimeout(t)
}

func (c *connection) Recycle() {

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

// connection implements transport.Connection
type connection struct {
	conn netpoll.Connection
	r    *reader
	w    *writer
}

var _ transport.Reader = (*reader)(nil)

// reader implements transport.Reader
type reader struct {
	delegate netpoll.Reader
}

var _ transport.Writer = (*writer)(nil)

// writer implements transport.Writer
type writer struct {
	delegate netpoll.Writer
}
