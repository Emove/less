package channel

import "time"

type Stater interface {
	GetChannel() *Channel
	GetIdleTime() time.Time
	GetLastRead() int64
	GetLastWrite() int64
}
