package framebuf

import (
	"fmt"
	"io"

	"github.com/emove/less/codec"
)

type reader struct {
	src io.Reader
	buf []byte
	pos int
}

func NewReader(src io.Reader) codec.ReaderBuffer {
	return &reader{
		src: src,
		buf: make([]byte, 0, 256),
	}
}

func (r *reader) Peek(n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	if err := r.ensure(n); err != nil {
		return nil, err
	}
	return r.buf[r.pos : r.pos+n], nil
}

func (r *reader) Next(n int) ([]byte, error) {
	buf, err := r.Peek(n)
	if err != nil {
		return nil, err
	}
	r.pos += n
	r.compact()
	return buf, nil
}

func (r *reader) Skip(n int) error {
	if _, err := r.Peek(n); err != nil {
		return err
	}
	r.pos += n
	r.compact()
	return nil
}

func (r *reader) Slice(n int) (codec.Frame, error) {
	buf, err := r.Peek(n)
	if err != nil {
		return nil, err
	}
	frame := NewFrame(buf)
	r.pos += n
	r.compact()
	return frame, nil
}

func (r *reader) Len() int {
	return len(r.buf) - r.pos
}

func (r *reader) Release() {
	r.src = nil
	r.buf = nil
	r.pos = 0
}

func (r *reader) ensure(n int) error {
	for len(r.buf)-r.pos < n {
		r.compact()
		tmp := make([]byte, max(n-(len(r.buf)-r.pos), 256))
		readN, err := r.src.Read(tmp)
		if readN > 0 {
			r.buf = append(r.buf, tmp[:readN]...)
		}
		if err != nil {
			if err == io.EOF && len(r.buf)-r.pos >= n {
				break
			}
			return err
		}
		if readN == 0 {
			return io.ErrUnexpectedEOF
		}
	}
	if len(r.buf)-r.pos < n {
		return fmt.Errorf("reader buffer underflow: have %d want %d", len(r.buf)-r.pos, n)
	}
	return nil
}

func (r *reader) compact() {
	if r.pos == 0 {
		return
	}
	if r.pos < len(r.buf)/2 && r.pos < 256 {
		return
	}
	unread := len(r.buf) - r.pos
	if unread > 0 {
		copy(r.buf[:unread], r.buf[r.pos:])
	}
	r.buf = r.buf[:unread]
	r.pos = 0
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
