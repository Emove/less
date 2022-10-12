package writer

import (
	"github.com/emove/less/internal/errors"
	less_io "github.com/emove/less/pkg/io"
	"io"
)

var ErrWriterBufferNotEnough = errors.New("residual buffer not enough")

const defaultBufferSize = 1 << 8

// WrapBufferWriter decorator will be regarded as a buff and write data directly
func WrapBufferWriter(decorator less_io.Writer) less_io.Writer {
	return &writer{
		decorator:     decorator,
		preWriteIndex: 0,
		writeIndex:    0,
		writeDirectly: true,
		growable:      false,
	}
}

// NewBufferWriter data will be wrote to decorator when Flush called
func NewBufferWriter(decorator io.Writer) less_io.Writer {
	return &writer{
		decorator:     decorator,
		preWriteIndex: 0,
		writeIndex:    0,
		writeDirectly: false,
		buff:          make([]byte, defaultBufferSize),
		growable:      true,
	}
}

// NewBufferWriterWithBuff writes data to the given buf
func NewBufferWriterWithBuff(buf []byte) less_io.Writer {
	return &writer{
		decorator:     nil,
		buff:          buf,
		preWriteIndex: 0,
		writeIndex:    0,
		writeDirectly: false,
		growable:      false,
	}
}

// writer implements transport.Writer
type writer struct {
	decorator     io.Writer
	buff          []byte
	writeDirectly bool // if true, write data to decorator directly
	preWriteIndex int
	writeIndex    int
	growable      bool // if true, the buff grow when remain space not enough
}

// Write writes buf to buffer directly
func (w *writer) Write(buf []byte) (n int, err error) {
	if w.writeDirectly {
		n, err = w.decorator.(less_io.Writer).Write(buf)
		if err == nil {
			w.writeIndex += n
		}
		return
	}

	need := len(buf)
	if !w.checkWriteable(need) {
		return 0, ErrWriterBufferNotEnough
	}

	copy(w.buff[w.writeIndex:w.writeIndex+need], buf)
	w.writeIndex += need

	return need, nil
}

// Malloc returns a slice containing the next n bytes from the buffer
func (w *writer) Malloc(n int) (buf []byte, err error) {
	if w.writeDirectly {
		w.writeIndex += n
		return w.decorator.(less_io.Writer).Malloc(n)
	}

	if !w.checkWriteable(n) {
		return nil, ErrWriterBufferNotEnough
	}
	buf = w.buff[w.writeIndex : w.writeIndex+n]
	w.writeIndex += n
	return buf, nil
}

// MallocLength returns the total length of the written data
// that has not yet been submitted in the writer
func (w *writer) MallocLength() (length int) {
	return w.writeIndex
}

// Flush writes all malloc data to net.Conn
func (w *writer) Flush() error {

	if w.writeDirectly {
		w.preWriteIndex = w.writeIndex
		if d, ok := w.decorator.(less_io.Writer); ok {
			return d.Flush()
		}
		return nil
	}

	if !w.growable {
		// data had wrote to the given buff
		return nil
	}

	if w.writeIndex <= 0 || w.preWriteIndex == w.writeIndex {
		// ignore this operation
		return nil
	}
	if _, err := w.decorator.Write(w.buff[w.preWriteIndex:w.writeIndex]); err != nil {
		return err
	}
	w.buff = w.buff[w.writeIndex:]
	w.preWriteIndex = w.writeIndex
	if d, ok := w.decorator.(less_io.Writer); ok {
		return d.Flush()
	}
	return nil
}

// Release releases the buffer and reuse writer
func (w *writer) Release() {
	w.decorator = nil
	w.buff = nil
	w.writeDirectly = false
	w.preWriteIndex = 0
	w.writeIndex = 0
}

func (w *writer) checkWriteable(n int) bool {
	if len(w.buff)-w.writeIndex >= n {
		return true
	}
	// buffer space not enough
	if !w.growable {
		return false
	}

	// try to grow by means of a reslice.
	if l := len(w.buff); n <= cap(w.buff)-l {
		w.buff = w.buff[:l+n]
		return true
	}
	min := w.writeIndex + n
	capacity := cap(w.buff)
	for capacity < min {
		capacity <<= 1
	}

	buf := make([]byte, capacity)
	copy(buf, w.buff)
	w.buff = buf
	return true
}
