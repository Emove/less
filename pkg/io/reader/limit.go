package reader

import (
	"github.com/emove/less/internal/errors"
	less_io "github.com/emove/less/pkg/io"
)

// NewLimitReader returns a Reader that readable bytes limited
func NewLimitReader(decorator less_io.Reader, limit uint32) less_io.Reader {
	return &limitReader{
		decorator: decorator,
		total:     int(limit),
		remain:    limit,
	}
}

type limitReader struct {
	remain    uint32
	total     int
	decorator less_io.Reader
}

var _ less_io.Reader = (*limitReader)(nil)

func (lr *limitReader) Read(buff []byte) (n int, err error) {
	if int(lr.remain)-len(buff) < 0 {
		return 0, errors.New("buffer remain not enough, remain: %d, want: %d", lr.remain, len(buff))
	}
	n, err = lr.decorator.Read(buff)
	if err == nil {
		lr.remain -= uint32(n)
	}
	return
}

func (lr *limitReader) Next(n int) (buf []byte, err error) {
	if int(lr.remain)-n < 0 {
		return buf, errors.New("buffer remain not enough, remain: %d, want: %d", lr.remain, n)
	}
	if buf, err = lr.decorator.Next(n); err != nil {
		return
	}
	lr.remain -= uint32(n)
	return
}

func (lr *limitReader) Peek(n int) (buf []byte, err error) {
	if int(lr.remain)-n < 0 {
		return buf, errors.New("buffer remain not enough, remain: %d, want: %d", lr.remain, n)
	}
	return lr.decorator.Peek(n)
}

func (lr *limitReader) Skip(n int) (err error) {
	if int(lr.remain)-n < 0 {
		return errors.New("buf remain not enough, remain: %d, want: %d", lr.remain, n)
	}
	if err = lr.decorator.Skip(n); err != nil {
		return
	}
	lr.remain -= uint32(n)
	return
}

// Length returns total readable size
func (lr *limitReader) Length() int {
	return lr.total
}

func (lr *limitReader) Release() {
	if lr.remain != 0 {
		_ = lr.decorator.Skip(int(lr.remain))
		lr.remain = 0
	}
	lr.decorator = nil
}
