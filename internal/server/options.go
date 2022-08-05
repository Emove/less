package server

import (
	"less/keepalive"
	"less/proto"
	"less/transport"
	"math"
	"time"
)

type ServerOptions struct {
	MaxConnectionSize     uint32
	MaxSendMessageSize    uint32
	MaxReceiveMessageSize uint32

	ReadTimeout time.Duration
	IdleTimeout time.Duration

	KeepaliveParams keepalive.ServerParameters

	Codec proto.Codec

	OnConn      func(con transport.Connection)
	OnConnClose func(con transport.Connection)
	OnMessage   func(request interface{}) (response interface{})
}

var DefaultServerOptions = ServerOptions{
	MaxConnectionSize:     math.MaxUint32,
	MaxSendMessageSize:    1024 * 1024 * 4,
	MaxReceiveMessageSize: 1024 * 1024 * 4,
}
