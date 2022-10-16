package state

import (
	"time"

	"github.com/emove/less/internal/channel"
)

// Stater is a abstraction to get channel states
type Stater interface {
	// Channel returns channel
	Channel() *channel.Channel
	// IdleTime returns channel's idle time
	IdleTime() time.Time
	// LastRead returns channel last read timestamp
	LastRead() int64
	// LastWrite returns channel last write timestamp
	LastWrite() int64
}
