package trans

import (
	"math"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/keepalive"
	"github.com/emove/less/router"
)

type Option func(ops *options)

type options struct {
	maxChannelSize        uint32
	maxSendMessageSize    uint32
	maxReceiveMessageSize uint32
	packetCodec           codec.PacketCodec
	payloadCodec          codec.PayloadCodec
	onChannel             []less.OnChannel
	onChannelClosed       []less.OnChannelClosed
	router                router.Router
	inbound               []less.Middleware
	outbound              []less.Middleware
	kp                    *keepalive.KeepaliveParameters
	useLessMsgCodec       bool
}

var defaultTransOptions = &options{
	maxChannelSize:        math.MaxUint32,  // infinity
	maxSendMessageSize:    1024 * 1024 * 4, // 4M
	maxReceiveMessageSize: 1024 * 1024 * 4, // 4M
	packetCodec:           packet.NewVariableLengthCodec(),
	payloadCodec:          payload.NewTextCodec(),
	kp: &keepalive.KeepaliveParameters{ // infinity
		HealthParams: &keepalive.HealthParams{},
		GoAwayParams: &keepalive.GoAwayParams{},
	},
}

func MaxChannelSize(size uint32) Option {
	return func(ops *options) {
		ops.maxChannelSize = size
	}
}

func MaxSendMessageSize(size uint32) Option {
	return func(ops *options) {
		ops.maxSendMessageSize = size
	}
}

func MaxReceiveMessageSize(size uint32) Option {
	return func(ops *options) {
		ops.maxReceiveMessageSize = size
	}
}

func WithPacketCodec(codec codec.PacketCodec) Option {
	return func(ops *options) {
		ops.packetCodec = codec
	}
}

func WithPayloadCodec(codec codec.PayloadCodec) Option {
	return func(ops *options) {
		ops.payloadCodec = codec
	}
}

func AddOnChannel(onChannel ...less.OnChannel) Option {
	return func(ops *options) {
		ops.onChannel = append(ops.onChannel, onChannel...)
	}
}

func AddOnChannelClosed(onChannelClosed ...less.OnChannelClosed) Option {
	return func(ops *options) {
		ops.onChannelClosed = append(ops.onChannelClosed, onChannelClosed...)
	}
}

func AddInboundMiddleware(inbound ...less.Middleware) Option {
	return func(ops *options) {
		ops.inbound = append(ops.inbound, inbound...)
	}
}

func AddOutboundMiddleware(outbound ...less.Middleware) Option {
	return func(ops *options) {
		ops.outbound = append(ops.outbound, outbound...)
	}
}

func WithRouter(router router.Router) Option {
	return func(ops *options) {
		ops.router = router
	}
}

func Keepalive(kp keepalive.KeepaliveParameters) Option {
	return func(ops *options) {
		// judge whether using inner msg
		if kp.HealthParams != nil && kp.HealthParams.Ping != nil {
			if _, ok := kp.HealthParams.Ping.(*keepalive.Ping); ok {
				ops.useLessMsgCodec = true
			}
		}
		if !ops.useLessMsgCodec && kp.GoAwayParams != nil && kp.GoAwayParams.GoAway != nil {
			if _, ok := kp.GoAwayParams.GoAway.(*keepalive.GoAway); ok {
				ops.useLessMsgCodec = true
			}
		}
		ops.kp = &kp
	}
}
