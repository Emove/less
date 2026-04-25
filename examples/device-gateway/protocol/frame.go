package protocol

import "github.com/emove/less/codec"

type bytesFrame struct {
	data []byte
}

var _ codec.Frame = (*bytesFrame)(nil)

func NewFrame(p []byte) codec.Frame {
	cp := append([]byte(nil), p...)
	return &bytesFrame{data: cp}
}

func (f *bytesFrame) Bytes() []byte {
	if f == nil {
		return nil
	}
	return f.data
}

func (*bytesFrame) Retain() {}

func (*bytesFrame) Release() {}
