package transport

import (
	"math"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/pkg/router"
)

type Option func(ops *Options)

type Options struct {
	idleTimeout time.Duration

	maxConnectionSize     uint32
	maxSendMessageSize    uint32
	maxReceiveMessageSize uint32

	packetCodec  codec.PacketCodec
	payloadCodec codec.PayloadCodec

	onChannel       []less.OnChannel
	onChannelClosed []less.OnChannelClosed

	router router.Router

	inbound  []less.Middleware
	outbound []less.Middleware
}

var defaultTransOptions = &Options{
	idleTimeout: time.Second * 30,

	// don't limit
	maxConnectionSize: math.MaxUint32,
	// default 4M
	maxSendMessageSize:    1024 * 1024 * 4,
	maxReceiveMessageSize: 1024 * 1024 * 4,

	packetCodec:  packet.NewVariableLengthCodec(),
	payloadCodec: payload.NewTextCodec(),
}

func WithIdleTime(d time.Duration) Option {
	return func(ops *Options) {
		ops.idleTimeout = d
	}
}

func WithMaxConnectionSize(size uint32) Option {
	return func(ops *Options) {
		ops.maxConnectionSize = size
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
