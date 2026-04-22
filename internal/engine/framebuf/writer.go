package framebuf

import (
	"errors"
	"io"

	"github.com/emove/less/codec"
)

var ErrUnsupportedWriterBuffer = errors.New("unsupported writer buffer implementation")

type writer struct {
	segments [][]byte
}

func NewWriter() codec.WriterBuffer {
	return &writer{}
}

func (w *writer) Malloc(n int) ([]byte, error) {
	buf := make([]byte, n)
	w.segments = append(w.segments, buf)
	return buf, nil
}

func (w *writer) WriteBinary(p []byte) error {
	cp := append([]byte(nil), p...)
	w.segments = append(w.segments, cp)
	return nil
}

func (w *writer) WriteFrame(f codec.Frame) error {
	return w.WriteBinary(f.Bytes())
}

func (w *writer) Append(src codec.WriterBuffer) error {
	other, ok := src.(*writer)
	if !ok {
		if src == nil {
			return nil
		}
		return ErrUnsupportedWriterBuffer
	}
	for _, seg := range other.segments {
		cp := append([]byte(nil), seg...)
		w.segments = append(w.segments, cp)
	}
	other.Release()
	return nil
}

func (w *writer) Len() int {
	total := 0
	for _, seg := range w.segments {
		total += len(seg)
	}
	return total
}

func (w *writer) FlushTo(dst io.Writer) error {
	for _, seg := range w.segments {
		for len(seg) > 0 {
			n, err := dst.Write(seg)
			if err != nil {
				return err
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			seg = seg[n:]
		}
	}
	return nil
}

func (w *writer) Release() {
	w.segments = nil
}
