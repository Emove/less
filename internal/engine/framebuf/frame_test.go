package framebuf

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/emove/less/codec"
)

func TestFrame_RetainReleaseKeepsBytesStable(t *testing.T) {
	frame := NewFrame([]byte("hello"))
	frame.Retain()

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("Bytes() = %q, want %q", got, "hello")
	}

	frame.Release()
	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("Bytes() after first Release = %q, want %q", got, "hello")
	}

	frame.Release()
	if got := frame.Bytes(); got != nil {
		t.Fatalf("Bytes() after final Release = %v, want nil", got)
	}
}

func TestFrame_DuplicateReleaseDoesNotCorruptState(t *testing.T) {
	frame := NewFrame([]byte("safe"))
	frame.Release()
	frame.Release()

	if got := frame.Bytes(); got != nil {
		t.Fatalf("Bytes() after duplicate release = %v, want nil", got)
	}
}

func TestReader_SliceReturnsStableFrameAcrossRelease(t *testing.T) {
	reader := NewReader(bytes.NewBufferString("hello"))

	head, err := reader.Peek(2)
	if err != nil {
		t.Fatalf("Peek() error = %v", err)
	}
	if got := string(head); got != "he" {
		t.Fatalf("Peek() = %q, want %q", got, "he")
	}

	frame, err := reader.Slice(5)
	if err != nil {
		t.Fatalf("Slice() error = %v", err)
	}
	defer frame.Release()

	reader.Release()

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("frame.Bytes() = %q, want %q", got, "hello")
	}
}

func TestWriter_AppendAndFlushPreservesSegmentOrder(t *testing.T) {
	dst := NewWriter()
	src := NewWriter()

	header, err := dst.Malloc(4)
	if err != nil {
		t.Fatalf("Malloc() error = %v", err)
	}
	copy(header, []byte{0, 0, 0, 5})

	if err := src.WriteBinary([]byte("hello")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}
	if err := dst.Append(src); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	var out bytes.Buffer
	if err := dst.FlushTo(&out); err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}

	want := []byte{0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o'}
	if got := out.Bytes(); !reflect.DeepEqual(got, want) {
		t.Fatalf("FlushTo() wrote %v, want %v", got, want)
	}
}

func TestReader_CompactsConsumedPrefix(t *testing.T) {
	rd := NewReader(bytes.NewBufferString("hello world")).(*reader)

	if _, err := rd.Next(6); err != nil {
		t.Fatalf("Next() error = %v", err)
	}

	if got := rd.pos; got != 0 {
		t.Fatalf("pos after compaction = %d, want 0", got)
	}
	if got := string(rd.buf); got != "world" {
		t.Fatalf("buf after compaction = %q, want %q", got, "world")
	}
}

type shortWriter struct {
	maxPerWrite int
	buf         bytes.Buffer
}

func (w *shortWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := w.maxPerWrite
	if n > len(p) {
		n = len(p)
	}
	_, _ = w.buf.Write(p[:n])
	return n, nil
}

func TestWriter_FlushToHandlesShortWrites(t *testing.T) {
	w := NewWriter()
	if err := w.WriteBinary([]byte("hello world")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}

	dst := &shortWriter{maxPerWrite: 3}
	if err := w.FlushTo(dst); err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}

	if got := dst.buf.String(); got != "hello world" {
		t.Fatalf("short writer got %q, want %q", got, "hello world")
	}
}

type zeroWriter struct{}

func (zeroWriter) Write(p []byte) (int, error) { return 0, nil }

func TestWriter_FlushToRejectsZeroProgressWrites(t *testing.T) {
	w := NewWriter()
	if err := w.WriteBinary([]byte("hello")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}

	if err := w.FlushTo(zeroWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("FlushTo() error = %v, want %v", err, io.ErrShortWrite)
	}
}

type foreignWriterBuffer struct{}

func (foreignWriterBuffer) Malloc(n int) ([]byte, error)        { return make([]byte, n), nil }
func (foreignWriterBuffer) WriteBinary(p []byte) error          { return nil }
func (foreignWriterBuffer) WriteFrame(f codec.Frame) error      { return nil }
func (foreignWriterBuffer) Append(src codec.WriterBuffer) error { return nil }
func (foreignWriterBuffer) Len() int                            { return 0 }
func (foreignWriterBuffer) FlushTo(dst io.Writer) error         { return nil }
func (foreignWriterBuffer) Release()                            {}

func TestWriter_AppendRejectsUnsupportedWriterBuffer(t *testing.T) {
	dst := NewWriter()
	err := dst.Append(foreignWriterBuffer{})
	if !errors.Is(err, ErrUnsupportedWriterBuffer) {
		t.Fatalf("Append() error = %v, want %v", err, ErrUnsupportedWriterBuffer)
	}
}
