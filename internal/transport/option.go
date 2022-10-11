package transport

import (
	"math"
	"math/rand"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/internal/msg"
	"github.com/emove/less/keepalive"
	"github.com/emove/less/log"
	"github.com/emove/less/router"
)

type Option func(ops *Options)

type Options struct {
	maxChannelSize        uint32
	maxSendMessageSize    uint32
	maxReceiveMessageSize uint32

	packetCodec  codec.PacketCodec
	payloadCodec codec.PayloadCodec

	onChannel       []less.OnChannel
	onChannelClosed []less.OnChannelClosed

	router router.Router

	inbound  []less.Middleware
	outbound []less.Middleware

	kp              *keepalive.KeepaliveParameters
	useLessMsgCodec bool
}

var defaultTransOptions = &Options{
	maxChannelSize:        math.MaxUint32,  // infinity
	maxSendMessageSize:    1024 * 1024 * 4, // 4M
	maxReceiveMessageSize: 1024 * 1024 * 4, // 4M
	packetCodec:           packet.NewVariableLengthCodec(),
	payloadCodec:          payload.NewTextCodec(),
	kp: &keepalive.KeepaliveParameters{ // infinity
		HealthParams: &keepalive.HealthParams{
			Ping:           &keepalive.Ping{},
			Pong:           &keepalive.Pong{},
			PingRecognizer: defaultPingRecognizer,
			PongRecognizer: defaultPongRecognizer,
		},
		GoAwayParams: &keepalive.GoAwayParams{
			GoAway:           &keepalive.GoAway{},
			GoAwayRecognizer: defaultGoAwayRecognizer,
		},
	},
}

func WithMaxChannelSize(size uint32) Option {
	return func(ops *Options) {
		ops.maxChannelSize = size
	}
}

func WithMaxSendMessageSize(size uint32) Option {
	return func(ops *Options) {
		ops.maxSendMessageSize = size
	}
}

func WithMaxReceiveMessageSize(size uint32) Option {
	return func(ops *Options) {
		ops.maxReceiveMessageSize = size
	}
}

func WithPacketCodec(codec codec.PacketCodec) Option {
	return func(ops *Options) {
		ops.packetCodec = codec
	}
}

func WithPayloadCodec(codec codec.PayloadCodec) Option {
	return func(ops *Options) {
		ops.payloadCodec = codec
	}
}

func OnChannel(onChannel ...less.OnChannel) Option {
	return func(ops *Options) {
		ops.onChannel = append(ops.onChannel, onChannel...)
	}
}

func OnChannelClosed(onChannelClosed ...less.OnChannelClosed) Option {
	return func(ops *Options) {
		ops.onChannelClosed = append(ops.onChannelClosed, onChannelClosed...)
	}
}

func WithInbound(inbound ...less.Middleware) Option {
	return func(ops *Options) {
		ops.inbound = append(ops.inbound, inbound...)
	}
}

func WithOutbound(outbound ...less.Middleware) Option {
	return func(ops *Options) {
		ops.outbound = append(ops.outbound, outbound...)
	}
}

func WithRouter(router router.Router) Option {
	return func(ops *Options) {
		ops.router = router
	}
}

func Keepalive(kp keepalive.KeepaliveParameters) Option {
	return func(ops *Options) {

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
					ops.useLessMsgCodec = true
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
				ops.useLessMsgCodec = true
			}
		}

		ops.kp = &kp
	}
}

func getJitter(v time.Duration) time.Duration {
	// Generate a jitter between +/- 10% of the value.
	r := int64(v / 10)
	rd := rand.New(rand.NewSource(time.Now().UnixNano()))
	j := rd.Int63n(2*r) - r
	return time.Duration(j)
}
