package framebuf

import (
	"errors"
	"io"

	"github.com/emove/less/codec"
)

var ErrUnsupportedWriterBuffer = errors.New("unsupported writer buffer implementation")

type writer struct {
	alloc *allocator

	head *node
	tail *node

	length int

	frames   []codec.Frame
	released bool
}

func NewWriter() codec.WriterBuffer {
	return &writer{alloc: defaultAllocator}
}

func (w *writer) Malloc(n int) ([]byte, error) {
	if w == nil {
		return nil, ErrReleased
	}
	if w.released {
		return nil, ErrReleased
	}
	if n <= 0 {
		return nil, nil
	}

	nn := w.tail
	if nn == nil || len(nn.writable()) < n {
		nn = w.alloc.newNode(n)
		nn.readEnd = 0
		nn.writeEnd = 0
		w.appendNode(nn)
	}

	start := nn.writeEnd
	nn.writeEnd += n
	if nn.readEnd < nn.writeEnd {
		nn.readEnd = nn.writeEnd
	}
	w.length += n

	return nn.block.buf[start:nn.writeEnd:nn.writeEnd], nil
}

func (w *writer) WriteBinary(p []byte) error {
	if len(p) == 0 {
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
	if f == nil {
		return nil
	}
	if w == nil {
		return ErrReleased
	}
	if w.released {
		return ErrReleased
	}

	f.Retain()
	if err := w.WriteBinary(f.Bytes()); err != nil {
		f.Release()
		return err
	}
	w.frames = append(w.frames, f)
	return nil
}

func (w *writer) Append(src codec.WriterBuffer) error {
	if src == nil {
		return nil
	}
	if w == nil {
		return ErrReleased
	}
	if w.released {
		return ErrReleased
	}

	other, ok := src.(*writer)
	if !ok {
		return ErrUnsupportedWriterBuffer
	}
	if other == nil || other == w || other.released {
		return nil
	}

	if other.head != nil {
		if w.tail != nil {
			w.tail.next = other.head
		} else {
			w.head = other.head
		}
		w.tail = other.tail
		w.length += other.length
	}
	if len(other.frames) > 0 {
		w.frames = append(w.frames, other.frames...)
	}

	other.head = nil
	other.tail = nil
	other.length = 0
	other.frames = nil
	other.released = true
	return nil
}

func (w *writer) Len() int {
	if w == nil || w.released {
		return 0
	}
	return w.length
}

func (w *writer) FlushTo(dst io.Writer) error {
	if w == nil || w.released || w.head == nil {
		return nil
	}

	cur := w.head
	for cur != nil {
		if cur.block == nil || cur.readEnd <= cur.readStart {
			next := cur.next
			w.dropHead(cur, next)
			cur = next
			continue
		}

		data := cur.block.buf[cur.readStart:cur.readEnd]
		for len(data) > 0 {
			n, err := dst.Write(data)
			if n > 0 {
				cur.readStart += n
				w.length -= n
				data = data[n:]
			}
			if err != nil {
				if cur.readStart >= cur.readEnd {
					next := cur.next
					w.dropHead(cur, next)
					cur = next
				}
				return err
			}
			if n == 0 {
				return io.ErrShortWrite
			}
		}

		next := cur.next
		w.dropHead(cur, next)
		cur = next
	}

	w.head = nil
	w.tail = nil
	w.length = 0
	return nil
}

func (w *writer) Release() {
	if w == nil || w.released {
		return
	}
	w.released = true

	w.releaseNodes()
	w.releaseFrames()

	w.head = nil
	w.tail = nil
	w.length = 0
}

func (w *writer) appendNode(n *node) {
	if n == nil {
		return
	}
	if w.head == nil {
		w.head = n
		w.tail = n
		return
	}
	w.tail.next = n
	w.tail = n
}

func (w *writer) dropHead(cur, next *node) {
	if cur != nil {
		cur.next = nil
		cur.release()
	}
	w.head = next
	if next == nil {
		w.tail = nil
	}
}

func (w *writer) releaseNodes() {
	for cur := w.head; cur != nil; {
		next := cur.next
		cur.next = nil
		cur.release()
		cur = next
	}
}

func (w *writer) releaseFrames() {
	for _, frame := range w.frames {
		if frame != nil {
			frame.Release()
		}
	}
	w.frames = nil
}
