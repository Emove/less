package framebuf

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"sync"
	"testing"

	"github.com/emove/less/codec"
)

func TestFrame_SingleSpanSurvivesNodeRelease(t *testing.T) {
	a := newAllocator()
	n := a.newNode(16)
	copy(n.block.buf, []byte("hello"))
	n.readEnd = 5
	n.writeEnd = 5

	frame := newFrameFromSpans(a, []span{{node: n, start: 0, end: 5}})
	if got := n.refs.Load(); got != 2 {
		t.Fatalf("node refs after frame creation = %d, want 2", got)
	}
	defer frame.Release()

	n.release()
	if got := n.refs.Load(); got != 1 {
		t.Fatalf("node refs after release = %d, want 1", got)
	}

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("Bytes() = %q, want %q", got, "hello")
	}
}

func TestFrame_MultiSpanLazyFlattensAndReleaseIsIdempotent(t *testing.T) {
	a := newAllocator()
	left := a.newNode(16)
	right := a.newNode(16)
	copy(left.block.buf, []byte("hel"))
	copy(right.block.buf, []byte("lo"))
	left.readEnd = 3
	left.writeEnd = 3
	right.readEnd = 2
	right.writeEnd = 2

	frame := newFrameFromSpans(a, []span{
		{node: left, start: 0, end: 3},
		{node: right, start: 0, end: 2},
	})
	if got := left.refs.Load(); got != 2 {
		t.Fatalf("left refs after frame creation = %d, want 2", got)
	}
	if got := right.refs.Load(); got != 2 {
		t.Fatalf("right refs after frame creation = %d, want 2", got)
	}

	left.release()
	right.release()
	if got := left.refs.Load(); got != 1 {
		t.Fatalf("left refs after release = %d, want 1", got)
	}
	if got := right.refs.Load(); got != 1 {
		t.Fatalf("right refs after release = %d, want 1", got)
	}

	first := frame.Bytes()
	if got := string(first); got != "hello" {
		t.Fatalf("Bytes() = %q, want %q", got, "hello")
	}

	second := frame.Bytes()
	if got := string(second); got != "hello" {
		t.Fatalf("Bytes() on second call = %q, want %q", got, "hello")
	}
	if len(first) > 0 && len(second) > 0 && &first[0] != &second[0] {
		t.Fatal("Bytes() should reuse the flattened buffer")
	}

	frame.Release()
	frame.Release()

	if got := frame.Bytes(); got != nil {
		t.Fatalf("Bytes() after Release = %v, want nil", got)
	}
}

func TestFrame_NewFrameFromSpansNormalizesInvalidSpans(t *testing.T) {
	a := newAllocator()
	n := a.newReadonlyNode([]byte("hello"))

	frame := newFrameFromSpans(a, []span{
		{},
		{node: nil, start: 0, end: 5},
		{node: n, start: -3, end: 99},
		{node: n, start: 4, end: 4},
	})
	defer frame.Release()

	if got := n.refs.Load(); got != 2 {
		t.Fatalf("node refs after frame creation = %d, want 2", got)
	}

	n.release()

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("Bytes() = %q, want %q", got, "hello")
	}
}

func TestFrame_RetainReleaseIsRaceSafeByContract(t *testing.T) {
	frame := NewFrame([]byte("hello"))
	var wg sync.WaitGroup
	wg.Add(2)

	worker := func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			frame.Retain()
			frame.Release()
		}
	}

	go worker()
	go worker()
	wg.Wait()

	frame.Release()
	if got := frame.Bytes(); got != nil {
		t.Fatalf("Bytes() after final Release = %v, want nil", got)
	}
}

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

func TestReader_ConsumedPrefixKeepsUnreadBytesOnHeadNode(t *testing.T) {
	rd := NewReader(bytes.NewBufferString("hello world")).(*reader)

	if err := rd.Skip(6); err != nil {
		t.Fatalf("Skip() error = %v", err)
	}

	typ := reflect.TypeOf(*rd)
	if _, ok := typ.FieldByName("buf"); ok {
		t.Fatal("reader should not keep a shadow buffer field")
	}
	if _, ok := typ.FieldByName("pos"); ok {
		t.Fatal("reader should not keep a shadow buffer cursor")
	}
	if rd.head == nil {
		t.Fatal("reader head = nil, want unread bytes on head node")
	}
	if rd.tail != rd.head {
		t.Fatal("reader tail should still point at the unread head node")
	}
	if got := rd.Len(); got != 5 {
		t.Fatalf("Len() after Skip() = %d, want 5", got)
	}
	if got := rd.head.readStart; got != 6 {
		t.Fatalf("head.readStart after Skip() = %d, want 6", got)
	}
	if got := rd.head.readEnd; got != 11 {
		t.Fatalf("head.readEnd after Skip() = %d, want 11", got)
	}
	if got := string(rd.head.readable()); got != "world" {
		t.Fatalf("head readable bytes = %q, want %q", got, "world")
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

func TestAllocator_LargeBlocksAreUnmanaged(t *testing.T) {
	a := newAllocator()
	block := a.getBlock(largeBlockThreshold + 1)
	if !block.unmanaged {
		t.Fatal("large block should be unmanaged")
	}
	a.putBlock(block)
}

func TestAllocator_GetBlockAppliesMinimumBlockSize(t *testing.T) {
	a := newAllocator()

	for _, size := range []int{1, 512} {
		block := a.getBlock(size)
		if block.unmanaged {
			t.Fatalf("block for size %d should be managed", size)
		}
		if got := cap(block.buf); got < minBlockSize {
			t.Fatalf("cap(%d) = %d, want >= %d", size, got, minBlockSize)
		}
		a.putBlock(block)
	}
}

func TestNode_ReleaseIsIdempotentForNilAndReleasedNodes(t *testing.T) {
	var n *node
	n.retain()
	n.release()

	a := newAllocator()
	live := a.newNode(16)
	live.retain()
	live.release()
	live.release()
	live.release()
	live.release()
}

func TestNode_ResetClearsLinksAndOffsets(t *testing.T) {
	a := newAllocator()
	n := a.newNode(16)
	n.next = a.newNode(16)
	n.readStart = 2
	n.readEnd = 4
	n.writeEnd = 8
	n.release()

	reused := a.newNode(16)
	if reused.next != nil {
		t.Fatal("new node kept next pointer")
	}
	if reused.readStart != 0 || reused.readEnd != 0 || reused.writeEnd != 0 {
		t.Fatalf("new node offsets = %d/%d/%d, want zero", reused.readStart, reused.readEnd, reused.writeEnd)
	}
	reused.release()
}
