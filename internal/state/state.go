package state

import (
	"time"

	"github.com/emove/less/internal/channel"
)

// Stater is a abstraction to get channel states
type Stater interface {
	// GetChannel returns channel
	GetChannel() *channel.Channel
	// GetIdleTime returns channel's idle time
	GetIdleTime() time.Time
	// GetLastRead returns channel last read timestamp
	GetLastRead() int64
	// GetLastWrite returns channel last write timestamp
	GetLastWrite() int64
}
