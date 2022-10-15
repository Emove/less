package keepalive

import (
	"testing"
	"time"

	"github.com/emove/less/keepalive"
)

func TestKeepaliveServer_Empty(t *testing.T) {
	// closing channel after 20 seconds
	KeepaliveServer(nil)
}

func TestKeepaliveServer_IdleTime(t *testing.T) {
	kp := &keepalive.KeepaliveParameters{
		MaxChannelIdleTime: 3 * time.Second,
	}
	KeepaliveServer(kp)
}

func TestKeepaliveServer_MaxChannelAge(t *testing.T) {
	kp := &keepalive.KeepaliveParameters{
		MaxChannelAge: 10 * time.Second,
		CloseGrace:    1 * time.Second,
	}
	KeepaliveServer(kp)
}

func TestKeepaliveServer_health(t *testing.T) {
	kp := &keepalive.KeepaliveParameters{
		HealthParams: &keepalive.HealthParams{
			Time:    3 * time.Second,
			Timeout: 2 * time.Second,
			//Timeout: 1 * time.Second,
			Ping: ping,
			Pong: pong,
			PingRecognizer: func(message interface{}) bool {
				content, ok := message.(string)
				return ok && content == "ping"
			},
			PongRecognizer: func(message interface{}) bool {
				content, ok := message.(string)
				return ok && content == "pong"
			},
		},
	}
	KeepaliveServer(kp)
}

func TestKeepaliveServer_GoAway(t *testing.T) {
	kp := &keepalive.KeepaliveParameters{
		//MaxChannelIdleTime: 3 * time.Second,
		MaxChannelAge: 10 * time.Second,
		GoAwayParams: &keepalive.GoAwayParams{
			GoAway: goaway,
			GoAwayRecognizer: func(message interface{}) bool {
				content, ok := message.([]byte)
				return ok && string(content) == "go away"
			},
		},
	}
	KeepaliveServer(kp)
}
