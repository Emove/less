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

type retainedFrame struct {
	mu        sync.Mutex
	refs      atomic.Int32
	alloc     *allocator
	spans     []span
	flatBlock *block
	flat      []byte
	released  bool
}

func NewFrame(p []byte) codec.Frame {
	alloc := defaultAllocator
	n := alloc.newNode(len(p))
	if len(p) > 0 {
		copy(n.block.buf, p)
	}
	n.readEnd = len(p)
	n.writeEnd = len(p)

	frame := newFrameFromSpans(alloc, []span{{node: n, start: 0, end: len(p)}})
	n.release()
	return frame
}

func newFrameFromSpans(alloc *allocator, spans []span) codec.Frame {
	if alloc == nil {
		alloc = defaultAllocator
	}

	owned := append([]span(nil), spans...)
	for i := range owned {
		if owned[i].node != nil {
			owned[i].node.retain()
		}
	}

	f := &retainedFrame{
		alloc: alloc,
		spans: owned,
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
	switch len(f.spans) {
	case 0:
		return nil
	case 1:
		sp := f.spans[0]
		if sp.node == nil || sp.node.block == nil {
			return nil
		}
		return sp.node.block.buf[sp.start:sp.end:sp.end]
	}

	total := 0
	for _, sp := range f.spans {
		total += sp.end - sp.start
	}
	if total <= 0 {
		return nil
	}

	alloc := f.alloc
	if alloc == nil {
		alloc = defaultAllocator
	}
	f.flatBlock = alloc.getBlock(total)
	f.flat = f.flatBlock.buf[:total:total]

	offset := 0
	for _, sp := range f.spans {
		if sp.node == nil || sp.node.block == nil || sp.end <= sp.start {
			continue
		}
		offset += copy(f.flat[offset:], sp.node.block.buf[sp.start:sp.end])
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
	if f.flatBlock != nil {
		alloc.putBlock(f.flatBlock)
	}
	for _, sp := range f.spans {
		if sp.node != nil {
			sp.node.release()
		}
	}

	f.spans = nil
	f.flatBlock = nil
	f.flat = nil
	f.released = true
}
