//go:build windows
// +build windows

package conn

import (
	"context"
	"fmt"
	"less/pkg/transport"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

func wrapConnection(conn net.Conn) transport.Connection {
	ctx, cancelFunc := context.WithCancel(context.Background())
	con := &connection{
		rawConn:    conn,
		ctx:        ctx,
		cancelFunc: cancelFunc,
		//readBuffer: netpoll.NewLinkBuffer(),
		writeChan: make(chan *writeRequest, 1),
	}

	go con.waitWrite()

	return con
}

var _ transport.Connection = (*connection)(nil)

type connection struct {
	rawConn net.Conn

	ctx        context.Context
	cancelFunc context.CancelFunc

	//readBuffer *netpoll.LinkBuffer
	readMux   sync.Mutex
	writeChan chan *writeRequest

	readTimeout time.Duration
	idleTimeout time.Duration
}

func (c *connection) Reader() transport.Reader {
	return &reader{
		con: c,
	}
}

func (c *connection) Writer() transport.Writer {
	w := writerPool.Get().(*writer)
	w.con = c
	//w.writeBuffer = netpoll.NewLinkBuffer()
	return w
}

func (c *connection) Close() error {
	c.cancelFunc()
	return c.rawConn.Close()
}

func (c *connection) LocalAddr() net.Addr {
	return c.rawConn.LocalAddr()
}

func (c *connection) RemoteAddr() net.Addr {
	return c.rawConn.RemoteAddr()
}

func (c *connection) SetReadTimeout(t time.Duration) error {
	if t >= 0 {
		c.readTimeout = t
	}
	return nil
}

func (c *connection) SetIdleTimeout(t time.Duration) error {
	if t >= 0 {
		c.idleTimeout = t
	}
	return nil
}

const released = 1

// reader implements Reader
// fixme: handle read timeout for all read functions
type reader struct {
	con     *connection
	release uint32
}

func (r *reader) Next(n int) (buf []byte, err error) {
	err = r.blockRead(n)
	if err != nil {
		return
	}

	//return r.con.readBuffer.Next(n)
	return nil, nil
}

func (r *reader) Peek(n int) (buf []byte, err error) {
	err = r.blockRead(n)
	if err != nil {
		return
	}

	//return r.con.readBuffer.Peek(n)
	return nil, nil
}

func (r *reader) Skip(n int) (err error) {
	err = r.blockRead(n)
	if err != nil {
		return
	}

	//return r.con.readBuffer.Skip(n)
	return nil
}

func (r *reader) Until(delim byte) (line []byte, err error) {
	r.con.lockRead()
	defer r.con.unlockRead()

	if atomic.LoadUint32(&r.release) == released {
		return nil, fmt.Errorf("reader has been released")
	}

	//buffer := r.con.readBuffer
	//for n := buffer.Len(); n > 0; n-- {
	//	buf := make([]byte, 1)
	//	_, err = io.ReadFull(r.con.rawConn, buf)
	//
	//	line = append(line, buf[0])
	//
	//	if buf[0] == delim {
	//		return
	//	}
	//
	//	if n == 1 {
	//		n = 2
	//	}
	//}

	return
}

func (r *reader) Release() (err error) {
	r.con.lockRead()()
	defer r.con.unlockRead()()
	//if atomic.CompareAndSwapUint32(&r.release, 0, released) {
	//	return r.con.readBuffer.Release()
	//}
	return nil
}

func (r *reader) blockRead(n int) error {
	r.con.lockRead()()
	defer r.con.unlockRead()

	if atomic.LoadUint32(&r.release) == released {
		return fmt.Errorf("reader has been released")
	}

	//buffer := r.con.readBuffer
	//
	//l := buffer.Len()
	//if l >= n {
	//	return nil
	//}
	//
	//buf, err := buffer.Malloc(n - l)
	//if err != nil {
	//	return err
	//}
	//
	//_, err = io.ReadFull(r.con.rawConn, buf)
	//if err != nil {
	//	return err
	//}
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
	con *connection
	//writeBuffer *netpoll.LinkBuffer
}

func (w *writer) Write(buf []byte) (n int, err error) {
	//return w.writeBuffer.WriteBinary(buf)
	return
}

func (w *writer) Malloc(n int) (buf []byte) {
	//buf, _ = w.writeBuffer.Malloc(n)
	return
}

func (w *writer) MallocLength() (length int) {
	//return w.writeBuffer.MallocLen()
	return
}

func (w *writer) Flush() error {
	//if err := w.writeBuffer.Flush(); err != nil {
	//	return err
	//}
	//return w.con.write(w.writeBuffer.Bytes())
	return nil
}

func (w *writer) Release() {
	//_ = w.writeBuffer.Release()
}

func (c *connection) lockRead() func() {
	return func() {
		c.readMux.Lock()
	}
}

func (c *connection) unlockRead() func() {
	return func() {
		c.readMux.Unlock()
	}
}

type writeRequest struct {
	data    []byte
	errChan chan<- error
}

func (c *connection) write(data []byte) (err error) {
	errChan := make(chan error, 0)
	ctx := &writeRequest{
		data:    data,
		errChan: errChan,
	}
	go func() {
		c.writeChan <- ctx
	}()
	select {
	case err = <-errChan:
		close(errChan)
		return
	}
}

func (c *connection) waitWrite() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case ctx := <-c.writeChan:
			_, err := c.rawConn.Write(ctx.data)
			ctx.errChan <- err
		}
	}
}
