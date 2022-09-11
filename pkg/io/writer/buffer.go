package writer

import (
	less_io "github.com/emove/less/pkg/io"
	"io"
)

func NewBufferWriter(decorator io.Writer) less_io.Writer {
	return &writer{
		decorator:  decorator,
		writeIndex: 0,
		buff:       make([]byte, 1024),
	}
}

func NewBufferWriterWithBuf(decorator io.Writer, buf []byte) less_io.Writer {
	return &writer{
		decorator:  decorator,
		writeIndex: 0,
		buff:       buf,
	}
}

// writer implements transport.Writer
type writer struct {
	decorator  io.Writer
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
	if w.writeIndex <= 0 {
		// ignore this operation
		return nil
	}
	if _, err := w.decorator.Write(w.buff[:w.writeIndex]); err != nil {
		return err
	}
	w.buff = w.buff[:w.writeIndex]
	w.writeIndex = 0
	return nil
}

// Release releases the buffer and reuse writer
func (w *writer) Release() {
	w.decorator = nil
	w.buff = nil
	w.writeIndex = 0
}

func (w *writer) ensureWriteable(n int) {
	if len(w.buff)-w.writeIndex < n {
		buf := make([]byte, n)
		w.buff = append(w.buff, buf...)
	}
}
