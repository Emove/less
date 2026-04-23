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

type block struct {
	buf       []byte
	unmanaged bool
}

type allocator struct {
	nodePool   sync.Pool
	blockPools [len(blockBucketSizes)]sync.Pool
}

func newAllocator() *allocator {
	a := &allocator{}
	for i, bucketSize := range blockBucketSizes {
		size := bucketSize
		a.blockPools[i].New = func() any {
			return make([]byte, size)
		}
	}
	a.nodePool.New = func() any {
		return &node{allocator: a}
	}
	return a
}

func (a *allocator) newNode(size int) *node {
	n := a.getNode()
	block := a.getBlock(maxInt(size, minBlockSize))
	n.block = block
	n.span = span{end: len(block.buf)}
	n.readStart = 0
	n.readEnd = 0
	n.writeEnd = 0
	n.readonly = false
	n.refs.Store(1)
	return n
}

func (a *allocator) newReadonlyNode(p []byte) *node {
	n := a.getNode()
	n.block = &block{buf: p, unmanaged: true}
	n.span = span{end: len(p)}
	n.readStart = 0
	n.readEnd = len(p)
	n.writeEnd = len(p)
	n.readonly = true
	n.refs.Store(1)
	return n
}

func (a *allocator) getNode() *node {
	n, _ := a.nodePool.Get().(*node)
	if n == nil {
		n = &node{allocator: a}
	}
	n.allocator = a
	return n
}

func (a *allocator) putNode(n *node) {
	if n == nil {
		return
	}
	a.nodePool.Put(n)
}

func (a *allocator) getBlock(size int) *block {
	if size <= 0 {
		size = 1
	}
	if size > largeBlockThreshold {
		return &block{buf: make([]byte, size), unmanaged: true}
	}

	bucketIndex, bucketSize := bucketForSize(size)
	if bucketIndex < 0 {
		return &block{buf: make([]byte, size), unmanaged: true}
	}

	buf, _ := a.blockPools[bucketIndex].Get().([]byte)
	if cap(buf) < bucketSize {
		buf = make([]byte, bucketSize)
	} else {
		buf = buf[:bucketSize]
	}
	return &block{buf: buf}
}

func (a *allocator) putBlock(b *block) {
	if b == nil || b.unmanaged {
		return
	}
	bucketIndex, bucketSize := bucketForSize(cap(b.buf))
	if bucketIndex < 0 || cap(b.buf) != bucketSize {
		return
	}
	a.blockPools[bucketIndex].Put(b.buf[:bucketSize])
}

func bucketForSize(size int) (int, int) {
	for i, bucketSize := range blockBucketSizes {
		if size <= bucketSize {
			return i, bucketSize
		}
	}
	return -1, 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
