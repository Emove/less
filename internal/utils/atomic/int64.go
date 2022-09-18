package atomic

import "sync/atomic"

type AtomicInt64 int64

func (i *AtomicInt64) Inc() int64 {
	return atomic.AddInt64((*int64)(i), 1)
}

func (i *AtomicInt64) Dec() {
	atomic.AddInt64((*int64)(i), -1)
}

func (i *AtomicInt64) Value() int64 {
	return atomic.LoadInt64((*int64)(i))
}
