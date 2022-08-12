//go:build windows
// +build windows

package conn

import (
	"less/pkg/transport"
	"net"
	"sync"
	"time"
)

// WrapConnection wrap net.Conn to transport.Connection
func WrapConnection(conn net.Conn) transport.Connection {
	return &connection{
		delegate: conn,
	}
}

var _ transport.Connection = (*connection)(nil)

// connection implements transport.Connection
type connection struct {
	delegate net.Conn

	readTimeout time.Duration
}

// Reader returns a reader
func (c *connection) Reader() transport.Reader {
	r := readerPool.Get().(*reader)
	r.con = c
	r.timeout = c.readTimeout
	r.buff = make([]byte, 1024)
	r.readIndex = 0
	r.writeIndex = 0
	return r
}

// Writer returns a writer
func (c *connection) Writer() transport.Writer {
	w := writerPool.Get().(*writer)
	w.con = c
	w.buff = make([]byte, 1024)
	return w
}

// Close closes the net.Conn
func (c *connection) Close() error {
	return c.delegate.Close()
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

var readerPool = sync.Pool{
	New: func() interface{} { return &reader{} },
}

var _ transport.Reader = (*reader)(nil)

// reader implements Reader
// fixme: handle read timeout for all read functions
type reader struct {
	con        *connection
	timeout    time.Duration
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

func (r *reader) Length() int {
	return len(r.buff)
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
	readable := r.writeIndex - r.readIndex
	if readable >= n {
		// enough
		return nil
	}

	want := n - readable

	if len(r.buff)-r.writeIndex < want {
		r.growth(want + r.writeIndex - len(r.buff))
	}

	_, err := r.con.delegate.Read(r.buff[r.writeIndex : r.writeIndex+want])
	if err != nil {
		return err
	}
	r.writeIndex += want
	return nil
}

func (r *reader) growth(want int) {
	//l := len(r.buff)
	//l <<= 1
	//buf := make([]byte, l)
	//copy(buf, r.buff)
	//
	//r.buff = buf
	// growing by slice default strategy
	buf := make([]byte, want)
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
	_, err := w.con.delegate.Write(w.buff[:w.writeIndex])
	return err
}

// Release releases the buffer and reuse writer
func (w *writer) Release() {
	w.con = nil
	w.buff = nil
	w.writeIndex = 0
}

func (w *writer) ensureWriteable(n int) {
	if len(w.buff)-w.writeIndex < n {
		buf := make([]byte, n)
		w.buff = append(w.buff, buf...)
	}
}
