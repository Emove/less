package reader

import (
	"github.com/emove/less/internal/errors"
	"sync"
	"time"

	less_io "github.com/emove/less/io"
	"io"
)

func NewBufferReader(decorator io.Reader) less_io.Reader {
	r := bufferReaderPool.Get().(*bufferReader)
	r.decorator = decorator
	r.buff = make([]byte, 1024)
	r.growable = true
	return r
}

func NewBufferReaderWithBuf(decorator io.Reader, buf []byte) less_io.Reader {
	r := bufferReaderPool.Get().(*bufferReader)
	r.decorator = decorator
	r.buff = buf
	r.growable = false
	return r
}

var bufferReaderPool = sync.Pool{
	New: func() interface{} { return &bufferReader{} },
}

// bufferReader implements Reader
type bufferReader struct {
	decorator  io.Reader
	timeout    time.Duration
	buff       []byte
	growable   bool
	readIndex  int
	writeIndex int
}

// Next returns the next n bytes
func (r *bufferReader) Next(n int) (buf []byte, err error) {
	buf, err = r.Peek(n)
	if err != nil {
		return
	}
	r.readIndex += n
	return
}

// Peek returns the next n bytes without advancing bufferReader
func (r *bufferReader) Peek(n int) (buf []byte, err error) {
	if err = r.ensureReadable(n); err != nil {
		return
	}

	return r.buff[r.readIndex : r.readIndex+n], nil
}

// Skip skips the next n bytes and advancing the bufferReader
func (r *bufferReader) Skip(n int) (err error) {
	_, err = r.Peek(n)
	if err != nil {
		return
	}
	r.readIndex += n
	return
}

// Until returns until the first occurrence of delim in the connection or an error occur
func (r *bufferReader) Until(delim byte) (line []byte, err error) {
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

func (r *bufferReader) Length() int {
	return len(r.buff)
}

// Release releases the bufferReader buffer and reuse bufferReader
func (r *bufferReader) Release() {
	r.decorator = nil
	r.buff = nil
	r.readIndex = 0
	r.writeIndex = 0
	bufferReaderPool.Put(r)
}

func (r *bufferReader) ensureReadable(n int) error {
	readable := r.writeIndex - r.readIndex
	if readable >= n {
		// enough
		return nil
	}

	want := n - readable
	remain := len(r.buff) - r.writeIndex
	if !r.growable && remain < want {
		return errors.New("given buffer not enough, remain: %d, need: %d", remain, want)
	}

	if remain < want {
		r.growth(want + r.writeIndex - len(r.buff))
	}

	_, err := r.decorator.Read(r.buff[r.writeIndex : r.writeIndex+want])
	if err != nil {
		return err
	}
	r.writeIndex += want
	return nil
}

func (r *bufferReader) growth(want int) {
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
