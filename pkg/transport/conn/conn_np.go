//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package conn

import (
	"github.com/cloudwego/netpoll"
	"github.com/cloudwego/netpoll/mux"
	"less/pkg/transport"
	"net"
	"sync"
	"time"
)

type connection struct {
	conn netpoll.Connection

	idleTimeout time.Duration

	sharedQueue *mux.ShardQueue
}

func wrapConnection(con netpoll.Connection) transport.Connection {
	return &connection{
		conn: con,
	}
}

func (c *connection) Reader() transport.Reader {
	return c.conn.Reader()
}

func (c *connection) Writer() transport.Writer {
	w := writerPool.Get().(*writer)
	w.sq = c.sharedQueue
	w.delegate = netpoll.NewLinkBuffer()
	return w
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

func (c *connection) SetIdleTimeout(t time.Duration) error {
	if t >= 0 {
		c.idleTimeout = t
	}
	return nil
}

var writerPool sync.Pool

var _ transport.Writer = (*writer)(nil)

func init() {
	writerPool.New = func() interface{} {
		return &writer{}
	}
}

type writer struct {
	delegate netpoll.Writer
	sq       *mux.ShardQueue
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

func (w *writer) Flush() {
	w.sq.Add(func() (buf netpoll.Writer, isNil bool) {
		return w.delegate, w.delegate.MallocLen() > 0
	})
}

func (w *writer) Release() {
	w.sq = nil
	w.delegate = nil
	writerPool.Put(w)
}

type reader struct {
	delegate netpoll.Reader
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

func (r *reader) Release() error {
	return r.delegate.Release()
}
