package reader

import (
	"github.com/emove/less/internal/errors"
	less_io "github.com/emove/less/pkg/io"
	"sync"
	"time"

	"io"
)

func NewBufferReader(decorator io.Reader) less_io.Reader {
	r := bufferReaderPool.Get().(*bufferReader)
	r.decorator = decorator
	r.buff = make([]byte, 256)
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

var _ less_io.Reader = (*bufferReader)(nil)

// bufferReader implements Reader
type bufferReader struct {
	decorator  io.Reader
	timeout    time.Duration
	buff       []byte
	growable   bool
	readIndex  int
	writeIndex int
}

// Read reads bytes start at readIndex
func (r *bufferReader) Read(buff []byte) (n int, err error) {
	if r.readIndex == r.writeIndex {
		n, err = r.decorator.Read(buff)
		if err != nil {
			return n, err
		}

		r.readIndex += n
		r.writeIndex += n
		return
	}

	if err = r.ensureReadable(len(buff), buff); err != nil {
		return 0, err
	}
	n = len(buff)
	r.readIndex += n

	return
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
	if n <= 0 {
		return
	}
	if err = r.ensureReadable(n, nil); err != nil {
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

// Length returns read bytes length
func (r *bufferReader) Length() int {
	return r.writeIndex
}

// Release releases the bufferReader buffer and reuse bufferReader
func (r *bufferReader) Release() {
	r.decorator = nil
	r.buff = nil
	r.readIndex = 0
	r.writeIndex = 0
	bufferReaderPool.Put(r)
}

func (r *bufferReader) ensureReadable(n int, buff []byte) (err error) {
	readable := r.writeIndex - r.readIndex
	if readable >= n {
		// enough
		if buff != nil {
			copy(buff, r.buff[r.readIndex:r.readIndex+n])
		}
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

	if buff != nil {
		if readable > 0 {
			copy(buff[:readable], r.buff[r.readIndex:r.readIndex+readable])
		}
		_, err = io.ReadFull(r.decorator, buff[readable:])
	} else {
		_, err = io.ReadFull(r.decorator, r.buff[r.writeIndex:r.writeIndex+want])
	}
	if err != nil {
		return err
	}
	r.writeIndex += want
	return nil
}

func (r *bufferReader) growth(want int) {
	// growing by slice default strategy
	//buf := make([]byte, want)
	//r.buff = append(r.buff, buf...)

	if l := len(r.buff); want <= cap(r.buff)-l {
		r.buff = r.buff[:l+want]
	}
	min := r.writeIndex + want
	capacity := cap(r.buff)
	for capacity < min {
		capacity <<= 1
	}

	buf := make([]byte, capacity)
	copy(buf, r.buff)
	r.buff = buf
}
