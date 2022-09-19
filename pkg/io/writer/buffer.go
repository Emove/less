package writer

import (
	less_io "github.com/emove/less/pkg/io"
	"io"
)

const defaultBufferSize = 1 << 8

// NewBufferWriter decorator will be regarded as a buff and write data directly
func NewBufferWriter(decorator less_io.Writer) less_io.Writer {
	return &writer{
		decorator:     decorator,
		preWriteIndex: 0,
		writeIndex:    0,
		writeDirectly: true,
	}
}

// NewBufferWriterWithBuf data will be wrote to decorator when Flush called
func NewBufferWriterWithBuf(decorator io.Writer) less_io.Writer {
	return &writer{
		decorator:     decorator,
		preWriteIndex: 0,
		writeIndex:    0,
		writeDirectly: false,
		// TODO optimize with bytes.Buffer
		buff: make([]byte, defaultBufferSize),
	}
}

// writer implements transport.Writer
type writer struct {
	decorator     io.Writer
	buff          []byte
	writeDirectly bool
	preWriteIndex int
	writeIndex    int
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
	w.ensureWriteable(need)

	copy(w.buff[w.writeIndex:w.writeIndex+need], buf)
	w.writeIndex += need

	return need, nil
}

// Malloc returns a slice containing the next n bytes from the buffer
func (w *writer) Malloc(n int) (buf []byte) {
	if w.writeDirectly {
		w.writeIndex += n
		return w.decorator.(less_io.Writer).Malloc(n)
	}

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

	if w.writeDirectly {
		w.preWriteIndex = w.writeIndex
		return w.decorator.(less_io.Writer).Flush()
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

func (w *writer) ensureWriteable(n int) {
	if len(w.buff)-w.writeIndex < n {
		buf := make([]byte, n)
		w.buff = append(w.buff, buf...)
	}
}
