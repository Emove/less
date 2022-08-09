//go:build windows
// +build windows

package conn

import (
	"context"
	"io"
	"less/pkg/transport"
	"net"
	"sync"
	"time"
)

// wrapConnection wrap net.Conn to transport.Connection
func wrapConnection(conn net.Conn) transport.Connection {
	ctx, cancelFunc := context.WithCancel(context.Background())
	con := &connection{
		rawConn:    conn,
		ctx:        ctx,
		cancelFunc: cancelFunc,
		writeChan:  make(chan *writeRequest, 1),
	}

	go con.waitWrite()

	return con
}

var _ transport.Connection = (*connection)(nil)

// connection implements transport.Connection
type connection struct {
	rawConn net.Conn

	ctx        context.Context
	cancelFunc context.CancelFunc

	// serialize write request
	writeChan chan *writeRequest

	readTimeout time.Duration
	idleTimeout time.Duration
}

// Reader returns a reader
func (c *connection) Reader() transport.Reader {
	r := readerPool.Get().(*reader)
	r.con = c
	r.buff = make([]byte, 4096)
	return r
}

// Writer returns a writer
func (c *connection) Writer() transport.Writer {
	w := writerPool.Get().(*writer)
	w.con = c
	return w
}

// Close closes the net.Conn
func (c *connection) Close() error {
	c.cancelFunc()
	return c.rawConn.Close()
}

// LocalAddr returns the local address
func (c *connection) LocalAddr() net.Addr {
	return c.rawConn.LocalAddr()
}

// RemoteAddr returns the remote address
func (c *connection) RemoteAddr() net.Addr {
	return c.rawConn.RemoteAddr()
}

// SetReadTimeout sets the timeout for future Read calls wait
func (c *connection) SetReadTimeout(t time.Duration) error {
	if t >= 0 {
		c.readTimeout = t
	}
	return nil
}

//SetIdleTimeout sets the idle timeout of connections.
func (c *connection) SetIdleTimeout(t time.Duration) error {
	if t >= 0 {
		c.idleTimeout = t
	}
	return nil
}

var readerPool = sync.Pool{
	New: func() interface{} {
		return &reader{}
	},
}

var _ transport.Reader = (*reader)(nil)

// reader implements Reader
// fixme: handle read timeout for all read functions
type reader struct {
	sync.Mutex
	con        *connection
	buff       []byte
	readIndex  int
	writeIndex int
}

// Next returns the next n bytes
func (r *reader) Next(n int) (buf []byte, err error) {
	buf, err = r.Peek(n)
	if err != nil {
		return
	}
	r.readIndex += n
	return
}

// Peek returns the next n bytes without advancing reader
func (r *reader) Peek(n int) (buf []byte, err error) {
	if err = r.ensureReadable(n); err != nil {
		return
	}

	return r.buff[r.readIndex : r.readIndex+n], nil
}

// Skip skips the next n bytes and advancing the reader
func (r *reader) Skip(n int) (err error) {
	_, err = r.Peek(n)
	if err != nil {
		return
	}
	r.readIndex += n
	return
}

// Until returns until the first occurrence of delim in the connection or an error occur
func (r *reader) Until(delim byte) (line []byte, err error) {
	var buf []byte
	start, cnt := r.readIndex, 1
	for {
		buf, err = r.Next(1)
		if err == nil && buf[0] != delim {
			cnt++
		} else {
			break
		}
	}
	if err == nil {
		line = r.buff[start : start+cnt]
	}
	return
}

// Release releases the reader buffer and reuse reader
func (r *reader) Release() {
	r.con = nil
	r.buff = nil
	r.readIndex = 0
	r.writeIndex = 0
	readerPool.Put(r)
}

func (r *reader) ensureReadable(n int) error {
	remain := cap(r.buff) - r.writeIndex
	if remain < n {
		r.growth(n - remain)
	}

	if r.writeIndex > r.readIndex {
		return nil
	}

	_, err := io.ReadFull(r.con.rawConn, r.buff[r.writeIndex:r.writeIndex+n])
	if err != nil {
		return err
	}
	r.writeIndex += n
	return nil
}

func (r *reader) growth(n int) {
	//newCap := cap(r.buff) + n
	//buf := make([]byte, newCap)
	//copy(buf, r.buff)
	//r.buff = buf

	// growing by slice default strategy
	buf := make([]byte, n+1)
	r.buff = append(r.buff, buf...)
}

var writerPool = sync.Pool{
	New: func() interface{} {
		return &writer{}
	},
}
var _ transport.Writer = (*writer)(nil)

// writer implements transport.Writer
type writer struct {
	con        *connection
	buff       []byte
	writeIndex int
}

// Write writes buf to buffer directly
func (w *writer) Write(buf []byte) (n int, err error) {
	need := len(buf)
	w.ensureWriteable(need)

	copy(w.buff[w.writeIndex:w.writeIndex+need], buf)
	w.writeIndex += need

	return need, nil
}

// Malloc returns a slice containing the next n bytes from the buffer
func (w *writer) Malloc(n int) (buf []byte) {
	w.ensureWriteable(n)
	buf = w.buff[w.writeIndex : w.writeIndex+n]
	w.writeIndex += n
	return buf
}

// MallocLength returns the total length of the written data
// that has not yet been submitted in the writer
func (w *writer) MallocLength() (length int) {
	return w.writeIndex
}

// Flush writes all malloc data to net.Conn
func (w *writer) Flush() error {
	return w.con.write(w.buff)
}

// Release releases the buffer and reuse writer
func (w *writer) Release() {
	w.con = nil
	w.buff = nil
	w.writeIndex = 0
	writerPool.Put(w)
}

func (w *writer) ensureWriteable(n int) {
	if len(w.buff)-w.writeIndex < n {
		buf := make([]byte, n)
		w.buff = append(w.buff, buf...)
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
	c.writeChan <- ctx
	for {
		select {
		case err = <-errChan:
			close(errChan)
			return
		}
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
