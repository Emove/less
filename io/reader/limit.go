package reader

import (
	"github.com/emove/less/internal/errors"
	"github.com/emove/less/io"
)

type limitReader struct {
	remain    uint32
	decorator io.Reader
}

func NewLimitReader(decorator io.Reader, limit uint32) io.Reader {
	return &limitReader{
		decorator: decorator,
		remain:    limit,
	}
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
		return buf, errors.New("buf remain not enough, remain: %d, want: %d", lr.remain, n)
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

func (lr *limitReader) Release() {
	if lr.remain != 0 {
		_ = lr.decorator.Skip(int(lr.remain))
		lr.remain = 0
	}
	lr.decorator.Release()
	lr.decorator = nil
}
