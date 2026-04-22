package framebuf

import "sync"

import "github.com/emove/less/codec"

type Frame interface {
	Bytes() []byte
	Retain()
	Release()
}

type retainedFrame struct {
	mu    sync.Mutex
	refs  int
	bytes []byte
}

func NewFrame(p []byte) codec.Frame {
	cp := append([]byte(nil), p...)
	return &retainedFrame{refs: 1, bytes: cp}
}

func (f *retainedFrame) Bytes() []byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bytes
}

func (f *retainedFrame) Retain() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refs > 0 {
		f.refs++
	}
}

func (f *retainedFrame) Release() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refs == 0 {
		return
	}
	f.refs--
	if f.refs == 0 {
		f.bytes = nil
	}
}
