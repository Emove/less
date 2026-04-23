# Less Framebuf Linkbuffer Optimization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `internal/engine/framebuf` internals with a simplified linkbuffer-style implementation using linked nodes, ref-counted frames, pooled allocation, and explicit lifecycle tests while preserving existing codec and engine contracts.

**Architecture:** Keep `codec.Frame`, `codec.ReaderBuffer`, and `codec.WriterBuffer` unchanged. Internally split `framebuf` into allocator, node, frame, reader, and writer responsibilities. Use `sync.Pool` only as an optimization; correctness comes from reference counts, idempotent release, and explicit ownership flags.

**Tech Stack:** Go 1.19, standard library `io`, `sync`, `sync/atomic`, existing Less codec and engine packages, `go test`, `go test -bench`.

---

## File Map

- Create: `internal/engine/framebuf/allocator.go`
  - Bucketed block allocation and node allocation helpers.
- Create: `internal/engine/framebuf/node.go`
  - Internal linked node type, span type, ownership flags, retain/release behavior.
- Create: `internal/engine/framebuf/errors.go`
  - Package errors used after release and for invalid buffer states.
- Modify: `internal/engine/framebuf/frame.go`
  - Replace copy-only retained frame with single-span and multi-span frame implementations.
- Modify: `internal/engine/framebuf/reader.go`
  - Replace single-slice compacting reader with linked-node reader.
- Modify: `internal/engine/framebuf/writer.go`
  - Replace segment-copy writer with linked-node writer.
- Modify: `internal/engine/framebuf/frame_test.go`
  - Keep existing contract tests and add frame/pool ownership tests.
- Create: `internal/engine/framebuf/reader_test.go`
  - Reader borrowed-slice, fragmented input, EOF, and cross-node tests.
- Create: `internal/engine/framebuf/writer_test.go`
  - Writer ownership, append, flush, and release tests.
- Create: `internal/engine/framebuf/bench_test.go`
  - Allocation and throughput benchmarks for representative hot paths.
- Modify: `codec/packet/var_length_test.go`
  - Add fragmented header/body coverage.
- Modify: `codec/packet/delimiter_test.go`
  - Add fragmented delimiter coverage.

## Task 1: Lock Reader Borrowing And Fragmentation Behavior

**Files:**
- Create: `internal/engine/framebuf/reader_test.go`
- Modify: `codec/packet/var_length_test.go`
- Modify: `codec/packet/delimiter_test.go`

- [ ] **Step 1: Add reader tests for cross-node contiguous reads and `Next` stability**

Create `internal/engine/framebuf/reader_test.go` with:

```go
package framebuf

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type chunkReader struct {
	chunks [][]byte
	index  int
	err    error
}

func newChunkReader(chunks ...string) *chunkReader {
	r := &chunkReader{chunks: make([][]byte, 0, len(chunks))}
	for _, chunk := range chunks {
		r.chunks = append(r.chunks, []byte(chunk))
	}
	return r
}

func (r *chunkReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		if r.err != nil {
			return 0, r.err
		}
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	return copy(p, chunk), nil
}

func TestReader_NextDoesNotCompactReturnedBorrowedSlice(t *testing.T) {
	rd := NewReader(bytes.NewBufferString("hello world"))

	first, err := rd.Next(6)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if got := string(first); got != "hello " {
		t.Fatalf("first Next() = %q, want %q", got, "hello ")
	}

	second, err := rd.Next(5)
	if err != nil {
		t.Fatalf("second Next() error = %v", err)
	}
	if got := string(second); got != "world" {
		t.Fatalf("second Next() = %q, want %q", got, "world")
	}
	if got := string(first); got != "hello " {
		t.Fatalf("borrowed first slice changed to %q after second read", got)
	}
}

func TestReader_PeekAndNextReturnContiguousBytesAcrossNodes(t *testing.T) {
	rd := newTestReaderWithBlockSize(newChunkReader("ab", "cd", "ef"), 2)

	peek, err := rd.Peek(5)
	if err != nil {
		t.Fatalf("Peek() error = %v", err)
	}
	if got := string(peek); got != "abcde" {
		t.Fatalf("Peek() = %q, want %q", got, "abcde")
	}

	next, err := rd.Next(5)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if got := string(next); got != "abcde" {
		t.Fatalf("Next() = %q, want %q", got, "abcde")
	}

	tail, err := rd.Next(1)
	if err != nil {
		t.Fatalf("tail Next() error = %v", err)
	}
	if got := string(tail); got != "f" {
		t.Fatalf("tail Next() = %q, want %q", got, "f")
	}
}

func TestReader_SliceAcrossNodesSurvivesReaderRelease(t *testing.T) {
	rd := newTestReaderWithBlockSize(newChunkReader("ab", "cd", "ef"), 2)

	frame, err := rd.Slice(6)
	if err != nil {
		t.Fatalf("Slice() error = %v", err)
	}
	defer frame.Release()

	rd.Release()

	if got := string(frame.Bytes()); got != "abcdef" {
		t.Fatalf("frame.Bytes() = %q, want %q", got, "abcdef")
	}
}

func TestReader_EOFBeforeRequestedBytesReturnsUnexpectedEOF(t *testing.T) {
	rd := NewReader(bytes.NewBufferString("abc"))

	_, err := rd.Peek(4)
	if !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		t.Fatalf("Peek() error = %v, want EOF-compatible error", err)
	}
}

func TestReader_ReleaseIsIdempotentAndRejectsFurtherReads(t *testing.T) {
	rd := NewReader(bytes.NewBufferString("abc"))
	rd.Release()
	rd.Release()

	if _, err := rd.Peek(1); !errors.Is(err, ErrReleased) {
		t.Fatalf("Peek() after Release error = %v, want %v", err, ErrReleased)
	}
}
```

- [ ] **Step 2: Add packet codec fragmented-input tests**

Append these tests to `codec/packet/var_length_test.go`:

```go
func TestVariableLengthCodec_DecodeFragmentedHeaderAndBody(t *testing.T) {
	reader := framebuf.NewReader(newTestReader([]byte{0, 0, 0, 5, 'h', 'e', 'l', 'l', 'o'}))

	frame, err := NewVariableLengthCodec().Decode(reader)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	defer frame.Release()

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("frame.Bytes() = %q, want %q", got, "hello")
	}
}
```

Append this helper and test to `codec/packet/delimiter_test.go`:

```go
type chunkedStringReader struct {
	chunks []string
	index  int
}

func (r *chunkedStringReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	return copy(p, chunk), nil
}

func TestDelimiterCodec_DecodeFragmentedDelimiter(t *testing.T) {
	codec := NewDelimiterCodec("\r\n", 32)
	reader := framebuf.NewReader(&chunkedStringReader{chunks: []string{"hel", "lo\r", "\n"}})

	frame, err := codec.Decode(reader)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	defer frame.Release()

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("frame.Bytes() = %q, want %q", got, "hello")
	}
}
```

- [ ] **Step 3: Run tests to verify current implementation exposes gaps**

Run:

```bash
go test ./internal/engine/framebuf -run 'TestReader_' -v
go test ./codec/packet -run 'TestVariableLengthCodec_DecodeFragmentedHeaderAndBody|TestDelimiterCodec_DecodeFragmentedDelimiter' -v
```

Expected: at least `TestReader_ReleaseIsIdempotentAndRejectsFurtherReads` fails because `ErrReleased` does not exist, and cross-node behavior cannot be forced until `newTestReaderWithBlockSize` exists.

- [ ] **Step 4: Commit failing reader contract tests**

```bash
git add internal/engine/framebuf/reader_test.go codec/packet/var_length_test.go codec/packet/delimiter_test.go
git commit -m "test: lock framebuf reader fragmentation contract"
```

## Task 2: Add Allocator And Node Primitives

**Files:**
- Create: `internal/engine/framebuf/errors.go`
- Create: `internal/engine/framebuf/allocator.go`
- Create: `internal/engine/framebuf/node.go`
- Modify: `internal/engine/framebuf/frame_test.go`

- [ ] **Step 1: Add allocator and node tests**

Append these tests to `internal/engine/framebuf/frame_test.go`:

```go
func TestAllocator_LargeBlocksAreUnmanaged(t *testing.T) {
	a := newAllocator()
	block := a.getBlock(largeBlockThreshold + 1)
	if !block.unmanaged {
		t.Fatal("large block should be unmanaged")
	}
	a.putBlock(block)
}

func TestNode_ReleaseReturnsOwnedBlockOnce(t *testing.T) {
	a := newAllocator()
	n := a.newNode(16)
	n.retain()
	n.release()
	n.release()
	n.release()
	if refs := n.refs.Load(); refs != 0 {
		t.Fatalf("refs = %d, want 0", refs)
	}
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
		t.Fatal("reused node kept next pointer")
	}
	if reused.readStart != 0 || reused.readEnd != 0 || reused.writeEnd != 0 {
		t.Fatalf("reused offsets = %d/%d/%d, want zero", reused.readStart, reused.readEnd, reused.writeEnd)
	}
	reused.release()
}
```

- [ ] **Step 2: Create package errors**

Create `internal/engine/framebuf/errors.go`:

```go
package framebuf

import "errors"

var ErrReleased = errors.New("framebuf: buffer has been released")
```

- [ ] **Step 3: Create allocator implementation**

Create `internal/engine/framebuf/allocator.go`:

```go
package framebuf

import "sync"

const (
	minBlockSize        = 4 << 10
	largeBlockThreshold = 64 << 10
)

var blockBucketSizes = [...]int{
	512,
	1 << 10,
	2 << 10,
	4 << 10,
	8 << 10,
	16 << 10,
	32 << 10,
	64 << 10,
}

type byteBlock struct {
	buf       []byte
	unmanaged bool
}

type allocator struct {
	nodePool sync.Pool
	blocks   [len(blockBucketSizes)]sync.Pool
}

func newAllocator() *allocator {
	a := &allocator{}
	a.nodePool.New = func() any { return &node{} }
	for i := range a.blocks {
		size := blockBucketSizes[i]
		a.blocks[i].New = func() any {
			return &byteBlock{buf: make([]byte, size)}
		}
	}
	return a
}

var defaultAllocator = newAllocator()

func (a *allocator) getBlock(size int) *byteBlock {
	if size <= 0 {
		size = minBlockSize
	}
	if size < minBlockSize {
		size = minBlockSize
	}
	if size > largeBlockThreshold {
		return &byteBlock{buf: make([]byte, size), unmanaged: true}
	}
	idx := bucketIndex(size)
	block := a.blocks[idx].Get().(*byteBlock)
	block.unmanaged = false
	block.buf = block.buf[:cap(block.buf)]
	return block
}

func (a *allocator) putBlock(block *byteBlock) {
	if block == nil || block.unmanaged {
		return
	}
	idx := bucketIndex(cap(block.buf))
	block.buf = block.buf[:cap(block.buf)]
	a.blocks[idx].Put(block)
}

func bucketIndex(size int) int {
	for i, bucket := range blockBucketSizes {
		if size <= bucket {
			return i
		}
	}
	return len(blockBucketSizes) - 1
}
```

- [ ] **Step 4: Create node implementation**

Create `internal/engine/framebuf/node.go`:

```go
package framebuf

import "sync/atomic"

type span struct {
	node  *node
	start int
	end   int
}

type node struct {
	alloc     *allocator
	block     *byteBlock
	readStart int
	readEnd   int
	writeEnd  int
	readonly  bool
	refs      atomic.Int32
	next      *node
}

func (a *allocator) newNode(size int) *node {
	n := a.nodePool.Get().(*node)
	block := a.getBlock(size)
	n.alloc = a
	n.block = block
	n.readStart = 0
	n.readEnd = 0
	n.writeEnd = 0
	n.readonly = false
	n.next = nil
	n.refs.Store(1)
	return n
}

func (a *allocator) newReadonlyNode(p []byte, unmanaged bool) *node {
	n := a.nodePool.Get().(*node)
	n.alloc = a
	n.block = &byteBlock{buf: p, unmanaged: unmanaged}
	n.readStart = 0
	n.readEnd = len(p)
	n.writeEnd = len(p)
	n.readonly = true
	n.next = nil
	n.refs.Store(1)
	return n
}

func (n *node) readable() int {
	if n == nil {
		return 0
	}
	return n.readEnd - n.readStart
}

func (n *node) writable() int {
	if n == nil || n.readonly || n.block == nil {
		return 0
	}
	return cap(n.block.buf) - n.writeEnd
}

func (n *node) bytes() []byte {
	return n.block.buf[n.readStart:n.readEnd:n.readEnd]
}

func (n *node) retain() {
	if n == nil {
		return
	}
	for {
		refs := n.refs.Load()
		if refs == 0 {
			return
		}
		if n.refs.CompareAndSwap(refs, refs+1) {
			return
		}
	}
}

func (n *node) release() {
	if n == nil {
		return
	}
	for {
		refs := n.refs.Load()
		if refs == 0 {
			return
		}
		if n.refs.CompareAndSwap(refs, refs-1) {
			if refs-1 == 0 {
				n.resetAndPut()
			}
			return
		}
	}
}

func (n *node) resetAndPut() {
	alloc := n.alloc
	block := n.block
	n.alloc = nil
	n.block = nil
	n.readStart = 0
	n.readEnd = 0
	n.writeEnd = 0
	n.readonly = false
	n.next = nil
	if alloc != nil {
		alloc.putBlock(block)
		alloc.nodePool.Put(n)
	}
}
```

- [ ] **Step 5: Run allocator/node tests**

Run:

```bash
go test ./internal/engine/framebuf -run 'TestAllocator_|TestNode_' -v
```

Expected: PASS.

- [ ] **Step 6: Commit allocator and node primitives**

```bash
git add internal/engine/framebuf/errors.go internal/engine/framebuf/allocator.go internal/engine/framebuf/node.go internal/engine/framebuf/frame_test.go
git commit -m "feat: add framebuf allocator and node primitives"
```

## Task 3: Replace Frame With Span-Backed Retained Frames

**Files:**
- Modify: `internal/engine/framebuf/frame.go`
- Modify: `internal/engine/framebuf/frame_test.go`

- [ ] **Step 1: Add frame tests for single and multi-span behavior**

Append to `internal/engine/framebuf/frame_test.go`:

```go
func TestFrame_SingleSpanSurvivesNodeRelease(t *testing.T) {
	a := newAllocator()
	n := a.newNode(16)
	copy(n.block.buf, "hello")
	n.readEnd = 5

	frame := newFrameFromSpans(a, []span{{node: n, start: 0, end: 5}})
	n.release()
	defer frame.Release()

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("Bytes() = %q, want %q", got, "hello")
	}
}

func TestFrame_MultiSpanLazyFlattensAndReleases(t *testing.T) {
	a := newAllocator()
	left := a.newNode(16)
	right := a.newNode(16)
	copy(left.block.buf, "hel")
	copy(right.block.buf, "lo")
	left.readEnd = 3
	right.readEnd = 2

	frame := newFrameFromSpans(a, []span{
		{node: left, start: 0, end: 3},
		{node: right, start: 0, end: 2},
	})
	left.release()
	right.release()

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("Bytes() = %q, want %q", got, "hello")
	}
	frame.Release()
	frame.Release()
	if got := frame.Bytes(); got != nil {
		t.Fatalf("Bytes() after Release = %v, want nil", got)
	}
}

func TestFrame_RetainReleaseIsRaceSafeByContract(t *testing.T) {
	frame := NewFrame([]byte("hello"))
	done := make(chan struct{}, 2)
	go func() {
		for i := 0; i < 1000; i++ {
			frame.Retain()
			frame.Release()
		}
		done <- struct{}{}
	}()
	go func() {
		for i := 0; i < 1000; i++ {
			frame.Retain()
			frame.Release()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
	frame.Release()
}
```

- [ ] **Step 2: Replace frame implementation**

Replace `internal/engine/framebuf/frame.go` with:

```go
package framebuf

import (
	"sync"
	"sync/atomic"

	"github.com/emove/less/codec"
)

type Frame interface {
	Bytes() []byte
	Retain()
	Release()
}

type retainedFrame struct {
	alloc     *allocator
	refs      atomic.Int32
	mu        sync.Mutex
	spans     []span
	flat      *byteBlock
	flatBytes []byte
	released  bool
}

func NewFrame(p []byte) codec.Frame {
	if len(p) == 0 {
		f := &retainedFrame{alloc: defaultAllocator}
		f.refs.Store(1)
		return f
	}
	block := defaultAllocator.getBlock(len(p))
	copy(block.buf, p)
	n := defaultAllocator.nodePool.Get().(*node)
	n.alloc = defaultAllocator
	n.block = block
	n.readStart = 0
	n.readEnd = len(p)
	n.writeEnd = len(p)
	n.readonly = false
	n.next = nil
	n.refs.Store(1)
	return newFrameFromSpans(defaultAllocator, []span{{node: n, start: 0, end: len(p)}})
}

func newFrameFromSpans(alloc *allocator, spans []span) codec.Frame {
	f := &retainedFrame{alloc: alloc, spans: append([]span(nil), spans...)}
	f.refs.Store(1)
	for _, sp := range f.spans {
		sp.node.retain()
	}
	return f
}

func (f *retainedFrame) Bytes() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.released {
		return nil
	}
	if len(f.spans) == 0 {
		return nil
	}
	if len(f.spans) == 1 && f.flatBytes == nil {
		sp := f.spans[0]
		return sp.node.block.buf[sp.start:sp.end:sp.end]
	}
	if f.flatBytes != nil {
		return f.flatBytes
	}
	total := 0
	for _, sp := range f.spans {
		total += sp.end - sp.start
	}
	f.flat = f.alloc.getBlock(total)
	f.flatBytes = f.flat.buf[:total:total]
	off := 0
	for _, sp := range f.spans {
		off += copy(f.flatBytes[off:], sp.node.block.buf[sp.start:sp.end])
	}
	return f.flatBytes
}

func (f *retainedFrame) Retain() {
	for {
		refs := f.refs.Load()
		if refs == 0 {
			return
		}
		if f.refs.CompareAndSwap(refs, refs+1) {
			return
		}
	}
}

func (f *retainedFrame) Release() {
	for {
		refs := f.refs.Load()
		if refs == 0 {
			return
		}
		if f.refs.CompareAndSwap(refs, refs-1) {
			if refs-1 == 0 {
				f.releaseFinal()
			}
			return
		}
	}
}

func (f *retainedFrame) releaseFinal() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.released {
		return
	}
	for _, sp := range f.spans {
		sp.node.release()
	}
	if f.flat != nil {
		f.alloc.putBlock(f.flat)
	}
	f.spans = nil
	f.flat = nil
	f.flatBytes = nil
	f.released = true
}
```

- [ ] **Step 3: Run frame tests including race-focused test**

Run:

```bash
go test ./internal/engine/framebuf -run 'TestFrame_' -v
go test -race ./internal/engine/framebuf -run TestFrame_RetainReleaseIsRaceSafeByContract -v
```

Expected: PASS.

- [ ] **Step 4: Commit span-backed frames**

```bash
git add internal/engine/framebuf/frame.go internal/engine/framebuf/frame_test.go
git commit -m "feat: make framebuf frames span-backed"
```

## Task 4: Implement Linked-Node Reader

**Files:**
- Modify: `internal/engine/framebuf/reader.go`
- Modify: `internal/engine/framebuf/reader_test.go`

- [ ] **Step 1: Add test-only constructor helper**

Append to `internal/engine/framebuf/reader_test.go`:

```go
func newTestReaderWithBlockSize(src io.Reader, blockSize int) *reader {
	return newReader(src, defaultAllocator, blockSize)
}
```

- [ ] **Step 2: Replace reader implementation**

Replace `internal/engine/framebuf/reader.go` with:

```go
package framebuf

import (
	"errors"
	"io"

	"github.com/emove/less/codec"
)

type reader struct {
	src       io.Reader
	alloc     *allocator
	blockSize int
	head      *node
	tail      *node
	length    int
	scratch   *byteBlock
	released  bool
}

func NewReader(src io.Reader) codec.ReaderBuffer {
	return newReader(src, defaultAllocator, minBlockSize)
}

func newReader(src io.Reader, alloc *allocator, blockSize int) *reader {
	if blockSize <= 0 {
		blockSize = minBlockSize
	}
	return &reader{src: src, alloc: alloc, blockSize: blockSize}
}

func (r *reader) Peek(n int) ([]byte, error) {
	if r.released {
		return nil, ErrReleased
	}
	if n <= 0 {
		return nil, nil
	}
	if err := r.ensure(n); err != nil {
		return nil, err
	}
	return r.contiguous(n), nil
}

func (r *reader) Next(n int) ([]byte, error) {
	buf, err := r.Peek(n)
	if err != nil {
		return nil, err
	}
	r.consume(n)
	return buf, nil
}

func (r *reader) Skip(n int) error {
	if r.released {
		return ErrReleased
	}
	if n <= 0 {
		return nil
	}
	if err := r.ensure(n); err != nil {
		return err
	}
	r.consume(n)
	return nil
}

func (r *reader) Slice(n int) (codec.Frame, error) {
	if r.released {
		return nil, ErrReleased
	}
	if n <= 0 {
		return NewFrame(nil), nil
	}
	if err := r.ensure(n); err != nil {
		return nil, err
	}
	spans := make([]span, 0, 2)
	remaining := n
	for remaining > 0 {
		cur := r.head
		if cur == nil {
			return nil, io.ErrUnexpectedEOF
		}
		readable := cur.readable()
		if readable == 0 {
			r.dropHead()
			continue
		}
		take := readable
		if take > remaining {
			take = remaining
		}
		start := cur.readStart
		end := start + take
		spans = append(spans, span{node: cur, start: start, end: end})
		cur.readStart = end
		r.length -= take
		remaining -= take
		if cur.readable() == 0 {
			r.dropHead()
		}
	}
	return newFrameFromSpans(r.alloc, spans), nil
}

func (r *reader) Len() int {
	return r.length
}

func (r *reader) Release() {
	if r.released {
		return
	}
	r.released = true
	for r.head != nil {
		next := r.head.next
		r.head.release()
		r.head = next
	}
	r.tail = nil
	r.length = 0
	r.src = nil
	if r.scratch != nil {
		r.alloc.putBlock(r.scratch)
		r.scratch = nil
	}
}

func (r *reader) ensure(n int) error {
	for r.length < n {
		if r.tail == nil || r.tail.writable() == 0 {
			r.appendNode(r.alloc.newNode(r.blockSize))
		}
		dst := r.tail.block.buf[r.tail.writeEnd:cap(r.tail.block.buf)]
		readN, err := r.src.Read(dst)
		if readN > 0 {
			r.tail.readEnd += readN
			r.tail.writeEnd += readN
			r.length += readN
		}
		if err != nil {
			if errors.Is(err, io.EOF) && r.length >= n {
				return nil
			}
			if errors.Is(err, io.EOF) && r.length < n {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		if readN == 0 {
			return io.ErrUnexpectedEOF
		}
	}
	return nil
}

func (r *reader) appendNode(n *node) {
	if r.head == nil {
		r.head = n
		r.tail = n
		return
	}
	r.tail.next = n
	r.tail = n
}

func (r *reader) contiguous(n int) []byte {
	if r.head.readable() >= n {
		start := r.head.readStart
		return r.head.block.buf[start:start+n:start+n]
	}
	if r.scratch != nil && cap(r.scratch.buf) < n {
		r.alloc.putBlock(r.scratch)
		r.scratch = nil
	}
	if r.scratch == nil {
		r.scratch = r.alloc.getBlock(n)
	}
	out := r.scratch.buf[:n:n]
	off := 0
	for cur := r.head; cur != nil && off < n; cur = cur.next {
		readable := cur.readable()
		if readable == 0 {
			continue
		}
		take := readable
		if take > n-off {
			take = n - off
		}
		off += copy(out[off:], cur.block.buf[cur.readStart:cur.readStart+take])
	}
	return out
}

func (r *reader) consume(n int) {
	remaining := n
	for remaining > 0 && r.head != nil {
		readable := r.head.readable()
		if readable == 0 {
			r.dropHead()
			continue
		}
		take := readable
		if take > remaining {
			take = remaining
		}
		r.head.readStart += take
		r.length -= take
		remaining -= take
		if r.head.readable() == 0 {
			r.dropHead()
		}
	}
}

func (r *reader) dropHead() {
	if r.head == nil || r.head.readable() != 0 {
		return
	}
	old := r.head
	r.head = old.next
	if r.tail == old {
		r.tail = r.head
	}
	old.next = nil
	old.release()
}
```

- [ ] **Step 3: Run reader and packet fragmented tests**

Run:

```bash
go test ./internal/engine/framebuf -run 'TestReader_' -v
go test ./codec/packet -run 'TestVariableLengthCodec_DecodeFragmentedHeaderAndBody|TestDelimiterCodec_DecodeFragmentedDelimiter' -v
```

Expected: PASS.

- [ ] **Step 4: Commit linked reader**

```bash
git add internal/engine/framebuf/reader.go internal/engine/framebuf/reader_test.go codec/packet/var_length_test.go codec/packet/delimiter_test.go
git commit -m "feat: implement linked-node framebuf reader"
```

## Task 5: Implement Linked-Node Writer

**Files:**
- Create: `internal/engine/framebuf/writer_test.go`
- Modify: `internal/engine/framebuf/writer.go`

- [ ] **Step 1: Add writer tests**

Create `internal/engine/framebuf/writer_test.go` with:

```go
package framebuf

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestWriter_WriteFrameRetainsCallerOwnership(t *testing.T) {
	frame := NewFrame([]byte("hello"))
	w := NewWriter()

	if err := w.WriteFrame(frame); err != nil {
		t.Fatalf("WriteFrame() error = %v", err)
	}
	frame.Release()

	var out bytes.Buffer
	if err := w.FlushTo(&out); err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("FlushTo() wrote %q, want %q", got, "hello")
	}
	w.Release()
}

func TestWriter_AppendTransfersSourceAndReleaseIsNoop(t *testing.T) {
	dst := NewWriter()
	src := NewWriter()
	if err := dst.WriteBinary([]byte("hello ")); err != nil {
		t.Fatalf("dst WriteBinary() error = %v", err)
	}
	if err := src.WriteBinary([]byte("world")); err != nil {
		t.Fatalf("src WriteBinary() error = %v", err)
	}
	if err := dst.Append(src); err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	src.Release()

	var out bytes.Buffer
	if err := dst.FlushTo(&out); err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}
	if got := out.String(); got != "hello world" {
		t.Fatalf("FlushTo() wrote %q, want %q", got, "hello world")
	}
}

func TestWriter_AppendFailureLeavesSourceWritable(t *testing.T) {
	dst := NewWriter()
	src := foreignWriterBuffer{}

	if err := dst.Append(src); !errors.Is(err, ErrUnsupportedWriterBuffer) {
		t.Fatalf("Append() error = %v, want %v", err, ErrUnsupportedWriterBuffer)
	}
}

type errAfterNWriter struct {
	max int
	buf bytes.Buffer
	err error
}

func (w *errAfterNWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n := w.max
	if n > len(p) {
		n = len(p)
	}
	_, _ = w.buf.Write(p[:n])
	return n, w.err
}

func TestWriter_FlushToHandlesPartialWriteWithError(t *testing.T) {
	w := NewWriter()
	if err := w.WriteBinary([]byte("hello")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}

	wantErr := errors.New("boom")
	dst := &errAfterNWriter{max: 2, err: wantErr}
	if err := w.FlushTo(dst); !errors.Is(err, wantErr) {
		t.Fatalf("FlushTo() error = %v, want %v", err, wantErr)
	}
	if got := dst.buf.String(); got != "he" {
		t.Fatalf("partial write = %q, want %q", got, "he")
	}
	w.Release()
}

func TestWriter_FlushSuccessClearsWrittenBytes(t *testing.T) {
	w := NewWriter()
	if err := w.WriteBinary([]byte("hello")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}

	var out bytes.Buffer
	if err := w.FlushTo(&out); err != nil {
		t.Fatalf("FlushTo() error = %v", err)
	}
	if err := w.FlushTo(&out); err != nil {
		t.Fatalf("second FlushTo() error = %v", err)
	}
	if got := out.String(); got != "hello" {
		t.Fatalf("FlushTo() wrote %q, want one copy", got)
	}
}

type zeroProgressWriter struct{}

func (zeroProgressWriter) Write([]byte) (int, error) { return 0, nil }

func TestWriter_FlushToRejectsZeroProgress(t *testing.T) {
	w := NewWriter()
	if err := w.WriteBinary([]byte("hello")); err != nil {
		t.Fatalf("WriteBinary() error = %v", err)
	}
	if err := w.FlushTo(zeroProgressWriter{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("FlushTo() error = %v, want %v", err, io.ErrShortWrite)
	}
	w.Release()
}
```

- [ ] **Step 2: Replace writer implementation**

Replace `internal/engine/framebuf/writer.go` with:

```go
package framebuf

import (
	"errors"
	"io"

	"github.com/emove/less/codec"
)

var ErrUnsupportedWriterBuffer = errors.New("unsupported writer buffer implementation")

type writer struct {
	alloc    *allocator
	head     *node
	tail     *node
	length   int
	frames   []codec.Frame
	released bool
}

func NewWriter() codec.WriterBuffer {
	return &writer{alloc: defaultAllocator}
}

func (w *writer) Malloc(n int) ([]byte, error) {
	if w.released {
		return nil, ErrReleased
	}
	if n <= 0 {
		return nil, nil
	}
	if w.tail == nil || w.tail.writable() < n {
		w.appendNode(w.alloc.newNode(n))
	}
	start := w.tail.writeEnd
	w.tail.writeEnd += n
	w.tail.readEnd = w.tail.writeEnd
	w.length += n
	return w.tail.block.buf[start:w.tail.writeEnd:w.tail.writeEnd], nil
}

func (w *writer) WriteBinary(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	if len(p) > largeBlockThreshold {
		cp := make([]byte, len(p))
		copy(cp, p)
		w.appendNode(w.alloc.newReadonlyNode(cp, true))
		w.length += len(cp)
		return nil
	}
	buf, err := w.Malloc(len(p))
	if err != nil {
		return err
	}
	copy(buf, p)
	return nil
}

func (w *writer) WriteFrame(f codec.Frame) error {
	if w.released {
		return ErrReleased
	}
	if f == nil {
		return nil
	}
	f.Retain()
	w.frames = append(w.frames, f)
	return w.WriteBinary(f.Bytes())
}

func (w *writer) Append(src codec.WriterBuffer) error {
	if w.released {
		return ErrReleased
	}
	other, ok := src.(*writer)
	if !ok {
		if src == nil {
			return nil
		}
		return ErrUnsupportedWriterBuffer
	}
	if other == nil || other.length == 0 {
		other.Release()
		return nil
	}
	if w.tail == nil {
		w.head = other.head
		w.tail = other.tail
	} else {
		w.tail.next = other.head
		w.tail = other.tail
	}
	w.length += other.length
	w.frames = append(w.frames, other.frames...)
	other.head = nil
	other.tail = nil
	other.length = 0
	other.frames = nil
	other.released = true
	return nil
}

func (w *writer) Len() int {
	return w.length
}

func (w *writer) FlushTo(dst io.Writer) error {
	if w.released {
		return ErrReleased
	}
	for w.head != nil {
		cur := w.head
		for cur.readStart < cur.readEnd {
			n, err := dst.Write(cur.block.buf[cur.readStart:cur.readEnd])
			if n > 0 {
				cur.readStart += n
				w.length -= n
			}
			if err != nil {
				return err
			}
			if n == 0 {
				return io.ErrShortWrite
			}
		}
		w.head = cur.next
		if w.tail == cur {
			w.tail = w.head
		}
		cur.next = nil
		cur.release()
	}
	w.releaseFrames()
	return nil
}

func (w *writer) Release() {
	if w.released {
		return
	}
	w.released = true
	for w.head != nil {
		next := w.head.next
		w.head.release()
		w.head = next
	}
	w.tail = nil
	w.length = 0
	w.releaseFrames()
}

func (w *writer) appendNode(n *node) {
	if w.head == nil {
		w.head = n
		w.tail = n
		return
	}
	w.tail.next = n
	w.tail = n
}

func (w *writer) releaseFrames() {
	for _, frame := range w.frames {
		frame.Release()
	}
	w.frames = nil
}
```

- [ ] **Step 3: Run writer tests**

Run:

```bash
go test ./internal/engine/framebuf -run 'TestWriter_' -v
```

Expected: PASS.

- [ ] **Step 4: Run all framebuf tests**

Run:

```bash
go test ./internal/engine/framebuf -v
```

Expected: PASS.

- [ ] **Step 5: Commit linked writer**

```bash
git add internal/engine/framebuf/writer.go internal/engine/framebuf/writer_test.go
git commit -m "feat: implement linked-node framebuf writer"
```

## Task 6: Add Benchmarks And Integration Verification

**Files:**
- Create: `internal/engine/framebuf/bench_test.go`
- Modify: no production files unless benchmarks expose a correctness bug from earlier tasks.

- [ ] **Step 1: Add framebuf benchmarks**

Create `internal/engine/framebuf/bench_test.go`:

```go
package framebuf

import (
	"bytes"
	"io"
	"testing"
)

func BenchmarkReaderSlice64B(b *testing.B) {
	payload := bytes.Repeat([]byte("a"), 64)
	for i := 0; i < b.N; i++ {
		rd := NewReader(bytes.NewReader(payload))
		frame, err := rd.Slice(len(payload))
		if err != nil {
			b.Fatal(err)
		}
		frame.Release()
		rd.Release()
	}
}

func BenchmarkReaderSlice1K(b *testing.B) {
	payload := bytes.Repeat([]byte("a"), 1024)
	for i := 0; i < b.N; i++ {
		rd := NewReader(bytes.NewReader(payload))
		frame, err := rd.Slice(len(payload))
		if err != nil {
			b.Fatal(err)
		}
		frame.Release()
		rd.Release()
	}
}

func BenchmarkReaderSlice64K(b *testing.B) {
	payload := bytes.Repeat([]byte("a"), 64<<10)
	for i := 0; i < b.N; i++ {
		rd := NewReader(bytes.NewReader(payload))
		frame, err := rd.Slice(len(payload))
		if err != nil {
			b.Fatal(err)
		}
		frame.Release()
		rd.Release()
	}
}

func BenchmarkReaderFragmentedPeek(b *testing.B) {
	for i := 0; i < b.N; i++ {
		rd := newTestReaderWithBlockSize(newChunkReader("ab", "cd", "ef", "gh"), 2)
		p, err := rd.Peek(8)
		if err != nil {
			b.Fatal(err)
		}
		if len(p) != 8 {
			b.Fatalf("len = %d, want 8", len(p))
		}
		rd.Release()
	}
}

func BenchmarkWriterWriteFrame(b *testing.B) {
	payload := NewFrame(bytes.Repeat([]byte("a"), 1024))
	defer payload.Release()
	for i := 0; i < b.N; i++ {
		w := NewWriter()
		if err := w.WriteFrame(payload); err != nil {
			b.Fatal(err)
		}
		if err := w.FlushTo(io.Discard); err != nil {
			b.Fatal(err)
		}
		w.Release()
	}
}

func BenchmarkWriterAppend(b *testing.B) {
	for i := 0; i < b.N; i++ {
		dst := NewWriter()
		src := NewWriter()
		if err := dst.WriteBinary([]byte("hello ")); err != nil {
			b.Fatal(err)
		}
		if err := src.WriteBinary([]byte("world")); err != nil {
			b.Fatal(err)
		}
		if err := dst.Append(src); err != nil {
			b.Fatal(err)
		}
		if err := dst.FlushTo(io.Discard); err != nil {
			b.Fatal(err)
		}
		dst.Release()
	}
}

func BenchmarkWriterFlushTo(b *testing.B) {
	payload := bytes.Repeat([]byte("a"), 4096)
	for i := 0; i < b.N; i++ {
		w := NewWriter()
		if err := w.WriteBinary(payload); err != nil {
			b.Fatal(err)
		}
		if err := w.FlushTo(io.Discard); err != nil {
			b.Fatal(err)
		}
		w.Release()
	}
}
```

- [ ] **Step 2: Run benchmarks with allocation reporting**

Run:

```bash
go test ./internal/engine/framebuf -bench . -benchmem
```

Expected: PASS and output includes `allocs/op` and `B/op` for each benchmark.

- [ ] **Step 3: Run full verification**

Run:

```bash
go test ./...
go test -race ./internal/engine/framebuf
```

Expected: PASS.

- [ ] **Step 4: Commit benchmarks and verified integration**

```bash
git add internal/engine/framebuf/bench_test.go
git commit -m "test: benchmark linkbuffer-style framebuf paths"
```

## Task 7: Final Cleanup And Contract Documentation

**Files:**
- Modify: `internal/engine/framebuf/frame.go`
- Modify: `internal/engine/framebuf/reader.go`
- Modify: `internal/engine/framebuf/writer.go`

- [ ] **Step 1: Add concise contract comments**

Ensure these comments exist above the relevant functions:

```go
// NewFrame copies caller-owned bytes into a retained frame. The returned
// frame owns its data until Release.
func NewFrame(p []byte) codec.Frame
```

```go
// Peek returns borrowed bytes that remain valid only until the next reader
// operation or Release. The returned slice must not be retained by callers.
func (r *reader) Peek(n int) ([]byte, error)
```

```go
// Next returns borrowed bytes and advances the reader. The returned slice
// remains valid only until the next reader operation or Release.
func (r *reader) Next(n int) ([]byte, error)
```

```go
// WriteFrame retains frame ownership for the writer lifetime. It does not
// transfer ownership from the caller; callers must still release their frame.
func (w *writer) WriteFrame(f codec.Frame) error
```

```go
// Append transfers source writer contents into this writer. On success the
// source writer becomes empty and Release on the source is a safe no-op.
func (w *writer) Append(src codec.WriterBuffer) error
```

- [ ] **Step 2: Run formatting**

Run:

```bash
gofmt -w internal/engine/framebuf/errors.go internal/engine/framebuf/allocator.go internal/engine/framebuf/node.go internal/engine/framebuf/frame.go internal/engine/framebuf/reader.go internal/engine/framebuf/writer.go internal/engine/framebuf/frame_test.go internal/engine/framebuf/reader_test.go internal/engine/framebuf/writer_test.go internal/engine/framebuf/bench_test.go codec/packet/var_length_test.go codec/packet/delimiter_test.go
```

Expected: command exits successfully.

- [ ] **Step 3: Run final verification**

Run:

```bash
go test ./...
go test -race ./internal/engine/framebuf
go test ./internal/engine/framebuf -bench . -benchmem
```

Expected: all tests pass; benchmarks report allocation metrics.

- [ ] **Step 4: Inspect git diff for accidental public contract changes**

Run:

```bash
git diff -- codec/codec.go transport/connection.go less.go
```

Expected: no diff. The implementation must preserve public codec and transport contracts.

- [ ] **Step 5: Commit final cleanup**

```bash
git add internal/engine/framebuf codec/packet/var_length_test.go codec/packet/delimiter_test.go
git commit -m "docs: clarify framebuf ownership contracts"
```

## Self-Review

Spec coverage:

- Linkbuffer-style linked nodes: Task 2, Task 4, Task 5.
- `sync.Pool` allocator and mcache-like buckets without external deps: Task 2.
- Ref-counted frames and lazy multi-span flattening: Task 3.
- Borrowed `Peek` and `Next` lifecycle: Task 1, Task 4, Task 7.
- `WriteFrame` retain semantics: Task 5, Task 7.
- `Append` transfer semantics: Task 5, Task 7.
- `FlushTo` short write and partial-error semantics: Task 5.
- Release idempotency: Task 1, Task 2, Task 3, Task 5.
- Fragmented codec integration: Task 1, Task 4, Task 6.
- Benchmarks with `allocs/op` and `B/op`: Task 6.
- Public contract preservation: Task 7.

Completion scan:

- No unresolved markers or unspecified edge-case steps remain.
- Every code-changing task includes concrete code or exact comments to add.
- Each verification step includes command and expected result.

Type consistency:

- `ErrReleased`, `allocator`, `node`, `span`, `reader`, and `writer` are introduced before subsequent tasks rely on them.
- `WriteFrame` uses retain semantics consistently with engine caller-side `defer payload.Release()`.
- `Append` transfer behavior is tested and documented consistently.
