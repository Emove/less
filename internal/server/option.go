package server

import (
	"github.com/emove/less"
	"github.com/emove/less/codec"
	"github.com/emove/less/internal/transport"
	router2 "github.com/emove/less/router"
	trans "github.com/emove/less/transport"
	"github.com/emove/less/transport/tcp"
)

var DefaultServerOptions = &Options{
	Addr:         "127.0.0.1",
	Port:         "8888",
	Transport:    tcp.New(),
	DisableGPool: false,
}

type Options struct {
	Addr               string
	Port               string
	Transport          trans.Transport
	PacketCodec        codec.PacketCodec
	PayloadCodec       codec.PayloadCodec
	Router             router2.Router
	OnChannels         []less.OnChannel
	OnChannelClosed    []less.OnChannelClosed
	InboundMiddleware  []less.Middleware
	OutboundMiddleware []less.Middleware
	TransOptions       []transport.Option
	DisableGPool       bool
}
