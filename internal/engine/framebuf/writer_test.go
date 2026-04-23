package framebuf

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/emove/less/codec"
)

type trackedFrame struct {
	buf      []byte
	retains  int
	releases int
}

func (f *trackedFrame) Bytes() []byte { return f.buf }
func (f *trackedFrame) Retain()       { f.retains++ }
func (f *trackedFrame) Release()      { f.releases++ }

type writerProxy struct {
	inner *writer
}

func (p writerProxy) Malloc(n int) ([]byte, error)        { return p.inner.Malloc(n) }
func (p writerProxy) WriteBinary(b []byte) error          { return p.inner.WriteBinary(b) }
func (p writerProxy) WriteFrame(f codec.Frame) error      { return p.inner.WriteFrame(f) }
func (p writerProxy) Append(src codec.WriterBuffer) error { return p.inner.Append(src) }
func (p writerProxy) Len() int                            { return p.inner.Len() }
func (p writerProxy) FlushTo(dst io.Writer) error         { return p.inner.FlushTo(dst) }
func (p writerProxy) Release()                            { p.inner.Release() }

type partialErrWriter struct {
	maxPerWrite int
	err         error
	buf         bytes.Buffer
}

func (w *partialErrWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := w.maxPerWrite
	if n <= 0 || n > len(p) {
		n = len(p)
	}
	_, _ = w.buf.Write(p[:n])
	if w.err != nil {
		return n, w.err
	}
	return n, nil
}

type zeroProgressWriter struct{}

func (zeroProgressWriter) Write(p []byte) (int, error) { return 0, nil }

func TestWriter_WriteFrameRetainsCallerOwnership(t *testing.T) {
	frame := &trackedFrame{buf: []byte("hello")}
	w := NewWriter().(*writer)

	if err := w.WriteFrame(frame); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	if got := frame.retains; got != 1 {
		t.Fatalf("frame retains = %d, want 1", got)
	}

	frame.Release()

	var out bytes.Buffer
	if err := w.FlushTo(&out); err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("FlushTo() = %q, want %q", got, "hello")
	}

	w.Release()
	if got := frame.releases; got != 2 {
		t.Fatalf("frame releases = %d, want 2", got)
	}
}

func TestWriter_AppendTransfersSourceAndReleaseIsNoop(t *testing.T) {
	frame := &trackedFrame{buf: []byte("hello")}
	src := NewWriter().(*writer)
	dst := NewWriter().(*writer)

	if err := src.WriteFrame(frame); err != nil {
		t.Fatalf("src.WriteFrame() error = %v", err)
	}
	if got := frame.retains; got != 1 {
		t.Fatalf("frame retains = %d, want 1", got)
	}

	if err := dst.Append(src); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if got := src.Len(); got != 0 {
		t.Fatalf("src.Len() = %d, want 0", got)
	}

	src.Release()
	src.Release()

	var out bytes.Buffer
	if err := dst.FlushTo(&out); err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("FlushTo() = %q, want %q", got, "hello")
	}

	dst.Release()
	if got := frame.releases; got != 1 {
		t.Fatalf("frame releases = %d, want 1", got)
	}
}

func TestWriter_AppendFailureLeavesSourceWritable(t *testing.T) {
	src := NewWriter().(*writer)
	if err := src.WriteBinary([]byte("hello")); err != nil {
		t.Fatalf("src.WriteBinary() error = %v", err)
	}
	dst := NewWriter().(*writer)
	proxy := writerProxy{inner: src}

	if err := dst.Append(proxy); !errors.Is(err, ErrUnsupportedWriterBuffer) {
		t.Fatalf("Append() error = %v, want %v", err, ErrUnsupportedWriterBuffer)
	}

	if err := src.WriteBinary([]byte("!")); err != nil {
		t.Fatalf("src.WriteBinary() after failed Append error = %v", err)
	}
	var out bytes.Buffer
	if err := src.FlushTo(&out); err != nil {
		t.Fatalf("src.FlushTo() error = %v", err)
	}
	if got := out.String(); got != "hello!" {
		t.Fatalf("src.FlushTo() = %q, want %q", got, "hello!")
	}
}

func TestWriter_FlushToHandlesPartialWriteWithError(t *testing.T) {
	w := NewWriter().(*writer)
	if err := w.WriteBinary([]byte("abcdef")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}

	dst := &partialErrWriter{maxPerWrite: 3, err: io.ErrClosedPipe}
	err := w.FlushTo(dst)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("FlushTo() error = %v, want %v", err, io.ErrClosedPipe)
	}
	if got := dst.buf.String(); got != "abc" {
		t.Fatalf("partial writer got %q, want %q", got, "abc")
	}
	if got := w.Len(); got != 3 {
		t.Fatalf("Len() after partial flush = %d, want 3", got)
	}

	var rest bytes.Buffer
	if err := w.FlushTo(&rest); err != nil {
		t.Fatalf("second FlushTo() error = %v", err)
	}
	if got := rest.String(); got != "def" {
		t.Fatalf("second FlushTo() = %q, want %q", got, "def")
	}
}

func TestWriter_FlushSuccessClearsWrittenBytes(t *testing.T) {
	w := NewWriter().(*writer)
	if err := w.WriteBinary([]byte("hello")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}

	var out bytes.Buffer
	if err := w.FlushTo(&out); err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("FlushTo() = %q, want %q", got, "hello")
	}
	if got := w.Len(); got != 0 {
		t.Fatalf("Len() after FlushTo() = %d, want 0", got)
	}

	var second bytes.Buffer
	if err := w.FlushTo(&second); err != nil {
		t.Fatalf("second FlushTo() error = %v", err)
	}
	if got := second.Len(); got != 0 {
		t.Fatalf("second FlushTo() wrote %d bytes, want 0", got)
	}
}

func TestWriter_FlushToRejectsZeroProgress(t *testing.T) {
	w := NewWriter().(*writer)
	if err := w.WriteBinary([]byte("hello")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}

	if err := w.FlushTo(zeroProgressWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("FlushTo() error = %v, want %v", err, io.ErrShortWrite)
	}
}
