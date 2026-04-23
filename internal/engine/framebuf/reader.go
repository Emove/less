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

	head *node
	tail *node

	length int
	buf    []byte
	pos    int

	scratch *block
	loan    *node

	released bool
}

func NewReader(src io.Reader) codec.ReaderBuffer {
	return newReader(src, defaultAllocator, minBlockSize)
}

func newReader(src io.Reader, alloc *allocator, blockSize int) *reader {
	if alloc == nil {
		alloc = defaultAllocator
	}
	if blockSize <= 0 {
		blockSize = minBlockSize
	}

	return &reader{
		src:       src,
		alloc:     alloc,
		blockSize: blockSize,
	}
}

// Peek returns borrowed bytes that remain valid until the next reader
// operation or Release.
func (r *reader) Peek(n int) ([]byte, error) {
	if err := r.checkReleased(); err != nil {
		return nil, err
	}
	if n <= 0 {
		r.finishOp(nil, nil)
		return nil, nil
	}
	if err := r.ensure(n); err != nil {
		r.finishOp(nil, nil)
		return nil, err
	}

	if r.head != nil {
		available := r.head.readEnd - r.head.readStart
		if n <= available {
			buf := r.head.block.buf[r.head.readStart : r.head.readStart+n : r.head.readStart+n]
			r.finishOp(r.head, nil)
			return buf, nil
		}
	}

	scratch := r.copyBytes(n)
	r.finishOp(nil, scratch)
	return scratch.buf[:n:n], nil
}

// Next returns borrowed bytes that remain valid until the next reader
// operation or Release, and advances the reader by n bytes.
func (r *reader) Next(n int) ([]byte, error) {
	if err := r.checkReleased(); err != nil {
		return nil, err
	}
	if n <= 0 {
		r.finishOp(nil, nil)
		return nil, nil
	}
	if err := r.ensure(n); err != nil {
		r.finishOp(nil, nil)
		return nil, err
	}

	if r.head != nil {
		available := r.head.readEnd - r.head.readStart
		if n <= available {
			loan := r.head
			buf := loan.block.buf[loan.readStart : loan.readStart+n : loan.readStart+n]
			loan.retain()
			r.consume(n)
			r.finishOpRetainedLoan(loan, nil)
			return buf, nil
		}
	}

	scratch := r.copyBytes(n)
	r.consume(n)
	r.finishOp(nil, scratch)
	return scratch.buf[:n:n], nil
}

func (r *reader) Skip(n int) error {
	if err := r.checkReleased(); err != nil {
		return err
	}
	if n <= 0 {
		r.finishOp(nil, nil)
		return nil
	}
	if err := r.ensure(n); err != nil {
		r.finishOp(nil, nil)
		return err
	}

	r.consume(n)
	r.finishOp(nil, nil)
	return nil
}

func (r *reader) Slice(n int) (codec.Frame, error) {
	if err := r.checkReleased(); err != nil {
		return nil, err
	}
	if n <= 0 {
		r.finishOp(nil, nil)
		return NewFrame(nil), nil
	}
	if err := r.ensure(n); err != nil {
		r.finishOp(nil, nil)
		return nil, err
	}

	spans, err := r.collectSpans(n)
	if err != nil {
		r.finishOp(nil, nil)
		return nil, err
	}
	frame := newFrameFromSpans(r.alloc, spans)
	r.consume(n)
	r.finishOp(nil, nil)
	return frame, nil
}

func (r *reader) Len() int {
	if r == nil || r.released {
		return 0
	}
	return r.length
}

func (r *reader) Release() {
	if r == nil || r.released {
		return
	}
	r.released = true

	if r.loan != nil {
		r.loan.release()
		r.loan = nil
	}
	if r.scratch != nil {
		r.alloc.putBlock(r.scratch)
		r.scratch = nil
	}

	for cur := r.head; cur != nil; {
		next := cur.next
		cur.next = nil
		cur.release()
		cur = next
	}

	r.head = nil
	r.tail = nil
	r.length = 0
	r.buf = nil
	r.pos = 0
	r.src = nil
}

func (r *reader) checkReleased() error {
	if r == nil || r.released {
		return ErrReleased
	}
	return nil
}

func (r *reader) ensure(n int) error {
	if n <= 0 {
		return nil
	}
	for r.length < n {
		if r.src == nil {
			return io.ErrUnexpectedEOF
		}

		chunk, err := r.readChunk()
		if chunk != nil {
			r.appendNode(chunk)
		}

		switch {
		case err == nil:
			if chunk == nil {
				return io.ErrUnexpectedEOF
			}
		case errors.Is(err, io.EOF):
			if r.length >= n {
				return nil
			}
			return io.ErrUnexpectedEOF
		default:
			return err
		}
	}
	return nil
}

func (r *reader) readChunk() (*node, error) {
	size := r.blockSize
	if size <= 0 {
		size = minBlockSize
	}

	n := r.alloc.newNode(size)
	limit := size
	if limit > len(n.block.buf) {
		limit = len(n.block.buf)
	}
	n.span.end = limit

	buf := n.writable()
	if len(buf) == 0 {
		n.release()
		return nil, io.ErrUnexpectedEOF
	}

	readN, err := r.src.Read(buf)
	if readN > 0 {
		n.readEnd = readN
		n.writeEnd = readN
	} else {
		n.release()
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	if err != nil {
		if errors.Is(err, io.EOF) {
			return n, io.EOF
		}
		return n, err
	}
	return n, nil
}

func (r *reader) appendNode(n *node) {
	if n == nil {
		return
	}
	if r.head == nil {
		r.head = n
		r.tail = n
	} else {
		r.tail.next = n
		r.tail = n
	}
	r.length += n.readEnd - n.readStart
}

func (r *reader) consume(n int) {
	for n > 0 && r.head != nil {
		cur := r.head
		available := cur.readEnd - cur.readStart
		if available <= 0 {
			r.head = cur.next
			if r.head == nil {
				r.tail = nil
			}
			cur.next = nil
			cur.release()
			continue
		}

		take := n
		if take > available {
			take = available
		}
		cur.readStart += take
		r.length -= take
		n -= take

		if cur.readStart < cur.readEnd {
			r.snapshotUnread()
			return
		}

		r.head = cur.next
		if r.head == nil {
			r.tail = nil
		}
		cur.next = nil
		cur.release()
	}

	r.snapshotUnread()
}

func (r *reader) collectSpans(n int) ([]span, error) {
	spans := make([]span, 0, 1)
	cur := r.head
	remaining := n

	for remaining > 0 {
		for cur != nil && cur.readEnd <= cur.readStart {
			cur = cur.next
		}
		if cur == nil {
			return nil, io.ErrUnexpectedEOF
		}

		available := cur.readEnd - cur.readStart
		take := remaining
		if take > available {
			take = available
		}
		spans = append(spans, span{
			node:  cur,
			start: cur.readStart,
			end:   cur.readStart + take,
		})
		remaining -= take
		cur = cur.next
	}

	return spans, nil
}

func (r *reader) copyBytes(n int) *block {
	if n <= 0 {
		return nil
	}

	buf := r.alloc.getBlock(n)
	if len(buf.buf) < n {
		buf.buf = make([]byte, n)
	}
	buf.buf = buf.buf[:n]

	offset := 0
	cur := r.head
	remaining := n
	for remaining > 0 && cur != nil {
		for cur != nil && cur.readEnd <= cur.readStart {
			cur = cur.next
		}
		if cur == nil {
			break
		}

		chunk := cur.readable()
		if len(chunk) > remaining {
			chunk = chunk[:remaining]
		}
		offset += copy(buf.buf[offset:], chunk)
		remaining -= len(chunk)
		cur = cur.next
	}

	if offset < n {
		buf.buf = buf.buf[:offset]
	}
	return buf
}

func (r *reader) finishOp(newLoan *node, newScratch *block) {
	if newLoan != nil {
		newLoan.retain()
	}
	r.finishOpRetainedLoan(newLoan, newScratch)
}

func (r *reader) finishOpRetainedLoan(newLoan *node, newScratch *block) {
	if r == nil || r.released {
		return
	}

	oldLoan := r.loan
	oldScratch := r.scratch

	r.loan = newLoan
	r.scratch = newScratch

	if oldLoan != nil {
		oldLoan.release()
	}
	if oldScratch != nil {
		r.alloc.putBlock(oldScratch)
	}
}

func (r *reader) snapshotUnread() {
	if r == nil || r.released {
		return
	}
	if r.length <= 0 {
		r.buf = r.buf[:0]
		r.pos = 0
		return
	}

	if cap(r.buf) < r.length {
		r.buf = make([]byte, r.length)
	} else {
		r.buf = r.buf[:r.length]
	}

	offset := 0
	for cur := r.head; cur != nil && offset < len(r.buf); cur = cur.next {
		if cur.readEnd <= cur.readStart {
			continue
		}
		offset += copy(r.buf[offset:], cur.readable())
	}
	r.pos = 0
}
