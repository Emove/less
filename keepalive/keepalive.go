package keepalive

import (
	"github.com/emove/less"
	"time"
)

// KeepaliveParameters is used to config channel keepalive and max-age parameters
type KeepaliveParameters struct {
	// MaxChannelIdleTime is a duration for the amount of time after which an
	// idle connection would be closed by sending a GoAway. Idleness duration is
	// defined since the most recent time the number of outstanding read or write
	// events became zero or the connection establishment. MaxChannelIdleTime only
	// works in server side.
	MaxChannelIdleTime time.Duration // the default value is infinity
	// MaxChannelAge is a duration for the maximum amount of time a
	// connection may exist before it will be closed by sending a GoAway. A
	// random jitter of +/-10% will be added to MaxChannelAge to spread out
	// connection storms.
	MaxChannelAge time.Duration // the default value is infinity
	// CloseGrace is an additive period after which the channel will be forcibly closed.
	CloseGrace   time.Duration // the default value is 10 seconds.
	HealthParams *HealthParams
	// If the GoAway not specific, server side will closing the channel forcibly
	GoAwayParams *GoAwayParams
}

// HealthParams defines channel health check parameters
type HealthParams struct {
	// After a duration of Time if the channel doesn't see any read activity it
	// pings the peer to see if the transport is still alive. If set below 1s,
	// a minimum value of 1s will be used instead.
	Time time.Duration // the default value is infinity
	// After having pinged for keepalive check, the channel waits for a duration
	// of Timeout and if no read activity is seen even after that the channel is
	// closed.
	Timeout time.Duration // the default value is 10 seconds
	// Sets the Ping message which can be handle correctly by the peer. If the peer
	// is Less, setting keepalive.Ping and ignore the following parameters. If the
	// Ping message is nil, the channel will be forcibly closed.
	Ping interface{} // the default value is nil
	// Sets a interceptor to recognize the custom Ping message
	PingRecognizer less.Interceptor // the default value is nil
	// Sets the Ping message which can be handle correctly by the peer.
	Pong interface{} // the default value is nil
	// Sets a interceptor to recognize the custom Pong message
	PongRecognizer less.Interceptor // the default value is nil
}

// GoAwayParams defines GoAway message type and interceptor
type GoAwayParams struct {
	// GoAway is a message type that used to tell client to close the channel actively.
	// If the peer is Less, keepalive.GoAway is a suggest value
	GoAway interface{}
	// GoAwayRecognizer is used to intercept the custom GoAway Message.
	// It only works on client side.
	GoAwayRecognizer less.Interceptor
}

// ============================== UNAVAILABLE NOW ================================ //

// Ping defines a Ping message which can be recognized by less service
type Ping struct{}

// Pong defines a Pong message which can be recognized by less service
type Pong struct{}

// GoAway defines a GoAway message which can be recognized by less service
type GoAway struct{}
