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

var defaultAllocator = newAllocator()

var frameChunkRegistry = struct {
	mu      sync.Mutex
	batches map[*allocator][][][]byte
}{
	batches: make(map[*allocator][][][]byte),
}

type retainedFrame struct {
	mu        sync.Mutex
	refs      atomic.Int32
	alloc     *allocator
	blocks    []*block
	chunks    [][]byte
	flatBlock *block
	flat      []byte
	released  bool
}

func NewFrame(p []byte) codec.Frame {
	return newFrameFromChunks(defaultAllocator, [][]byte{p})
}

func newFrameFromSpans(alloc *allocator, spans []span) codec.Frame {
	if alloc == nil {
		alloc = defaultAllocator
	}
	if len(spans) == 0 {
		f := &retainedFrame{alloc: alloc}
		f.refs.Store(1)
		return f
	}

	sources := takeFrameChunks(alloc)
	chunks := make([][]byte, 0, len(spans))
	for i, sp := range spans {
		var source []byte
		if i < len(sources) {
			source = sources[i]
		}
		if source != nil {
			start := sp.start
			if start < 0 {
				start = 0
			}
			if start > len(source) {
				start = len(source)
			}
			end := sp.end
			if end < start {
				end = start
			}
			if end > len(source) {
				end = len(source)
			}
			source = source[start:end]
		} else {
			size := sp.end - sp.start
			if size < 0 {
				size = 0
			}
			source = make([]byte, size)
		}
		chunks = append(chunks, source)
	}
	return newFrameFromChunks(alloc, chunks)
}

func registerFrameChunks(alloc *allocator, chunks ...[]byte) {
	if alloc == nil {
		alloc = defaultAllocator
	}
	batch := make([][]byte, len(chunks))
	for i, chunk := range chunks {
		batch[i] = append([]byte(nil), chunk...)
	}

	frameChunkRegistry.mu.Lock()
	frameChunkRegistry.batches[alloc] = append(frameChunkRegistry.batches[alloc], batch)
	frameChunkRegistry.mu.Unlock()
}

func takeFrameChunks(alloc *allocator) [][]byte {
	if alloc == nil {
		alloc = defaultAllocator
	}

	frameChunkRegistry.mu.Lock()
	defer frameChunkRegistry.mu.Unlock()

	batches := frameChunkRegistry.batches[alloc]
	if len(batches) == 0 {
		return nil
	}
	chunks := batches[0]
	if len(batches) == 1 {
		delete(frameChunkRegistry.batches, alloc)
	} else {
		frameChunkRegistry.batches[alloc] = batches[1:]
	}
	return chunks
}

func newFrameFromChunks(alloc *allocator, chunks [][]byte) codec.Frame {
	if alloc == nil {
		alloc = defaultAllocator
	}

	f := &retainedFrame{
		alloc:  alloc,
		blocks: make([]*block, 0, len(chunks)),
		chunks: make([][]byte, 0, len(chunks)),
	}
	for _, chunk := range chunks {
		b := alloc.getBlock(len(chunk))
		copy(b.buf, chunk)
		f.blocks = append(f.blocks, b)
		f.chunks = append(f.chunks, b.buf[:len(chunk):len(chunk)])
	}
	f.refs.Store(1)
	return f
}

func (f *retainedFrame) Bytes() []byte {
	if f == nil {
		return nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.released {
		return nil
	}
	if f.flat != nil {
		return f.flat
	}
	switch len(f.chunks) {
	case 0:
		return nil
	case 1:
		return f.chunks[0]
	}

	total := 0
	for _, chunk := range f.chunks {
		total += len(chunk)
	}
	if total == 0 {
		return nil
	}

	alloc := f.alloc
	if alloc == nil {
		alloc = defaultAllocator
	}
	f.flatBlock = alloc.getBlock(total)
	f.flat = f.flatBlock.buf[:total:total]

	offset := 0
	for _, chunk := range f.chunks {
		offset += copy(f.flat[offset:], chunk)
	}

	return f.flat
}

func (f *retainedFrame) Retain() {
	if f == nil {
		return
	}
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
	if f == nil {
		return
	}
	for {
		refs := f.refs.Load()
		if refs == 0 {
			return
		}
		if !f.refs.CompareAndSwap(refs, refs-1) {
			continue
		}
		if refs == 1 {
			f.releaseFinal()
		}
		return
	}
}

func (f *retainedFrame) releaseFinal() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.released {
		return
	}

	alloc := f.alloc
	if alloc == nil {
		alloc = defaultAllocator
	}
	for _, b := range f.blocks {
		alloc.putBlock(b)
	}
	if f.flatBlock != nil {
		alloc.putBlock(f.flatBlock)
	}

	f.blocks = nil
	f.chunks = nil
	f.flatBlock = nil
	f.flat = nil
	f.released = true
}
