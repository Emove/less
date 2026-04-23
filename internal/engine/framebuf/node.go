package framebuf

import "sync/atomic"

type span struct {
	node  *node
	start int
	end   int
}

type node struct {
	allocator *allocator
	block     *block
	span      span
	next      *node

	readStart int
	readEnd   int
	writeEnd  int

	readonly bool
	refs     atomic.Int32
}

func (n *node) readable() []byte {
	if n == nil || n.block == nil || n.readEnd <= n.readStart {
		return nil
	}
	return n.block.buf[n.readStart:n.readEnd]
}

func (n *node) writable() []byte {
	if n == nil || n.block == nil || n.readonly {
		return nil
	}
	end := n.span.end
	if end <= n.writeEnd {
		return nil
	}
	return n.block.buf[n.writeEnd:end]
}

func (n *node) bytes() []byte {
	return n.readable()
}

func (n *node) retain() {
	if n == nil {
		return
	}
	for {
		cur := n.refs.Load()
		if cur <= 0 {
			return
		}
		if n.refs.CompareAndSwap(cur, cur+1) {
			return
		}
	}
}

func (n *node) release() {
	if n == nil {
		return
	}
	for {
		cur := n.refs.Load()
		if cur <= 0 {
			return
		}
		if !n.refs.CompareAndSwap(cur, cur-1) {
			continue
		}
		if cur-1 > 0 {
			return
		}

		allocator := n.allocator
		block := n.block
		n.reset()
		if allocator != nil {
			allocator.putBlock(block)
			allocator.putNode(n)
		}
		return
	}
}

func (n *node) reset() {
	n.block = nil
	n.span = span{}
	n.next = nil
	n.readStart = 0
	n.readEnd = 0
	n.writeEnd = 0
	n.readonly = false
	n.refs.Store(0)
}
