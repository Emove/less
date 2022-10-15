package keepalive

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/internal/channel"
	"github.com/emove/less/internal/errors"
	"github.com/emove/less/internal/msg"
	"github.com/emove/less/internal/state"
	"github.com/emove/less/internal/timewheel"
	"github.com/emove/less/keepalive"
	"github.com/emove/less/log"
)

// Keepalive messages content
const (
	Ping   = "Ping"
	Pong   = "Pong"
	GoAway = "Go Away"
)

var (
	// defaultPingRecognizer used to recognize less's Ping message
	defaultPingRecognizer = func(message interface{}) bool {
		var lm *msg.LessMessage
		ok := false
		if lm, ok = message.(*msg.LessMessage); !ok {
			return false
		}
		return lm.MsgType == msg.Call && string(lm.Body) == Ping
	}

	// defaultPongRecognizer used to recognize less's Pong message
	defaultPongRecognizer = func(message interface{}) bool {
		var lm *msg.LessMessage
		ok := false
		if lm, ok = message.(*msg.LessMessage); !ok {
			return false
		}
		return lm.MsgType == msg.Reply && string(lm.Body) == Pong
	}

	// defaultGoAwayRecognizer used to recognize less's GoAway message
	defaultGoAwayRecognizer = func(message interface{}) bool {
		var lm *msg.LessMessage
		ok := false
		if lm, ok = message.(*msg.LessMessage); !ok {
			return false
		}
		body := string(lm.Body)
		return lm.MsgType == msg.Oneway && body == GoAway
	}
)

// KeepaliveMiddleware returns a middleware to intercept and handle keepalive messages
func KeepaliveMiddleware(kgetter func(ch *channel.Channel) *Keeper) less.Middleware {
	return func(handler less.Handler) less.Handler {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			k := kgetter(ch.(*channel.Channel))
			if k == nil {
				// did not running keepalive goroutine
				return handler(ctx, ch, message)
			}
			health := k.kp.HealthParams
			if health != nil {
				if health.PingRecognizer != nil && health.PingRecognizer(message) {
					return ch.Write(health.Pong)
				}
				if health.PongRecognizer != nil && health.PongRecognizer(message) {
					atomic.StoreInt64(&k.lastPong, time.Now().UnixNano())
					return nil
				}
			}

			// go away
			if k.kp.GoAwayParams != nil {
				goAwayRecognizer := k.kp.GoAwayParams.GoAwayRecognizer
				if goAwayRecognizer != nil && goAwayRecognizer(message) {
					_ = ch.Close(context.Background(), errors.New("closing channel due to received a go away message"))
					return nil
				}
			}

			return handler(ctx, ch, message)
		}
	}
}

// ConsummateKeepaliveParams consummates and checks keepalive parameters
func ConsummateKeepaliveParams(kp *keepalive.KeepaliveParameters) {
	if kp.MaxChannelAge > 0 {
		// add a jitter to MaxChannelAge.
		// inspired by grpc-go. https://github.com/grpc/grpc-go/blob/master/internal/transport/http2_server.go#224
		kp.MaxChannelAge += getJitter(kp.MaxChannelAge)
	}

	healthParams := kp.HealthParams
	if healthParams != nil && healthParams.Time > 0 {
		if healthParams.Time > 0 && healthParams.Time < time.Second {
			healthParams.Time = time.Second
		}
		if healthParams.Timeout <= 0 {
			healthParams.Timeout = 10 * time.Second
		}
		if healthParams.Ping == nil {
			log.Warnf("Keepalive params has set Time but without Ping-Pong params so that channels those does not see any activity after a duration of Time will be closed forcibly")
		} else {
			// if healthParams.Ping equals keepalive.Ping, completing else configs
			if _, ok := healthParams.Ping.(*keepalive.Ping); ok {
				healthParams.Ping = msg.NewMessage(msg.Call, Ping)
				healthParams.Pong = msg.NewMessage(msg.Reply, Pong)
				healthParams.PingRecognizer = defaultPingRecognizer
				healthParams.PongRecognizer = defaultPongRecognizer
			} else {
				if healthParams.Pong == nil {
					log.Warnf("Keepalive params has set Ping but without Pong so that channels those does not see any activity after a duration of Time will be closed forcibly")
				}
				if healthParams.PingRecognizer == nil {
					log.Warnf("Keepalive params has set Ping but without PingRecognizer so that channels those does not see any activity after a duration of Time will be closed forcibly")
				}
			}
		}
	}

	goAwayParams := kp.GoAwayParams
	if goAwayParams != nil && goAwayParams.GoAway != nil {
		if _, ok := goAwayParams.GoAway.(*keepalive.GoAway); ok {
			goAwayParams.GoAway = msg.NewMessage(msg.Oneway, GoAway)
			goAwayParams.GoAwayRecognizer = defaultGoAwayRecognizer
		}
	}
}

func NewKeeper(kp *keepalive.KeepaliveParameters, state state.Stater) *Keeper {
	return &Keeper{
		kp:    kp,
		state: state,
	}
}

type Keeper struct {
	kp       *keepalive.KeepaliveParameters
	state    state.Stater
	mu       sync.Mutex
	done     int32
	lastPing int64
	lastPong int64
}

func (k *Keeper) Keepalive() {

	kp := k.kp

	// max channel idle time
	if k.state.GetChannel().Side() == channel.Server && kp.MaxChannelIdleTime > 0 {
		var fn func()
		fn = func() {
			k.mu.Lock()
			if k.done == 1 {
				k.mu.Unlock()
				return
			}
			k.mu.Unlock()

			idleTime := k.state.GetIdleTime()
			if idleTime.IsZero() {
				// the channel is non-idle
				timewheel.Timer.AfterFunc(kp.MaxChannelIdleTime, fn)
				return
			}

			interval := kp.MaxChannelIdleTime - time.Since(idleTime)
			if interval <= 0 {
				log.Debugf("closing channel due to maximum idle time")
				k.goAwayChannel(errors.New("closing channel due to maximum idle time"))
				return
			}
			timewheel.Timer.AfterFunc(interval, fn)
		}
		timewheel.Timer.AfterFunc(kp.MaxChannelIdleTime, fn)
	}

	// max channel age
	if kp.MaxChannelAge > 0 {
		timewheel.Timer.AfterFunc(kp.MaxChannelAge, func() {
			k.mu.Lock()
			if k.done == 1 {
				k.mu.Unlock()
				return
			}
			k.mu.Unlock()
			log.Debugf("closing channel due to maximum keepalive age")
			k.goAwayChannel(errors.New("closing channel due to maximum Keepalive age"))
		})
	}

	// time
	healthParams := kp.HealthParams
	if healthParams != nil && healthParams.Time > 0 {
		var fn func()
		fn = func() {
			k.mu.Lock()
			if k.done == 1 {
				k.mu.Unlock()
				return
			}
			k.mu.Unlock()

			nowNano := time.Now().UnixNano()
			internal := nowNano - k.state.GetLastRead()
			if internal < int64(healthParams.Time) {
				timewheel.Timer.AfterFunc(healthParams.Time-time.Duration(internal), fn)
				return
			}

			ch := k.state.GetChannel()
			if ch.Readable() && internal >= int64(healthParams.Time) {
				if k.lastPing > atomic.LoadInt64(&k.lastPong) {
					// has sent ping before
					pingElapsed := nowNano - k.lastPing
					if pingElapsed >= int64(healthParams.Timeout) {
						// timeout
						log.Debugf("closing channel due to ping timeout")
						k.stopChannel(errors.New("closing channel due to ping timeout"))
						return
					}

					timewheel.Timer.AfterFunc(healthParams.Timeout-time.Duration(pingElapsed), fn)
					return
				}

				// try to send a ping
				if k.sendPing() {
					k.lastPing = time.Now().UnixNano()
					timewheel.Timer.AfterFunc(healthParams.Timeout, fn)
					return
				}
				// stop forcibly
				log.Debugf("closing channel due to ping failed")
				k.stopChannel(errors.New("closing channel due to pings failed"))
			}
		}
		timewheel.Timer.AfterFunc(healthParams.Time, fn)
	}
}

func (k *Keeper) Close() {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.done = 1
	log.Debugf("keeper closed")
}

func (k *Keeper) stopChannel(err error) {
	k.mu.Lock()
	// check whether the keeper is still working
	if k.done == 1 {
		k.mu.Unlock()
		return
	}
	k.done = 1
	k.mu.Unlock()

	ctx, cancelFunc := context.WithTimeout(context.Background(), k.kp.CloseGrace)
	defer cancelFunc()
	_ = k.state.GetChannel().Close(ctx, err)
}

func (k *Keeper) goAwayChannel(err error) {
	ch := k.state.GetChannel()
	params := k.kp.GoAwayParams
	if ch.Side() == channel.Server && params != nil && params.GoAway != nil {
		if e := ch.WriteDirectly(params.GoAway); e != nil {
			log.Errorf("send channel goaway message err: %v", e)
			// stop forcibly
			k.stopChannel(err)
		}
		log.Infof("channel will be closed by peer because of error: %v", err)
		return
	}
	// client's channel or go away msg not supported
	k.stopChannel(err)
}

func (k *Keeper) sendPing() bool {
	healthParams := k.kp.HealthParams
	if healthParams.Ping == nil || healthParams.Pong == nil || healthParams.PingRecognizer == nil || healthParams.PongRecognizer == nil {
		return false
	}
	err := k.state.GetChannel().WriteDirectly(healthParams.Ping)
	if err != nil {
		return false
	}
	return true
}

func getJitter(v time.Duration) time.Duration {
	// Generate a jitter between +/- 10% of the value.
	r := int64(v / 10)
	rd := rand.New(rand.NewSource(time.Now().UnixNano()))
	j := rd.Int63n(2*r) - r
	return time.Duration(j)
}