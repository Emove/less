package transport

import (
	"less/pkg/keepalive"
	"math"
	"time"
)

type TransServerOption struct {
	MaxConnectionSize     uint32
	MaxSendMessageSize    uint32
	MaxReceiveMessageSize uint32

	ReadTimeout time.Duration
	IdleTimeout time.Duration

	KeepaliveParams keepalive.ServerParameters

	Codec Codec

	OnConn      func(con Connection)
	OnConnClose func(con Connection)

	OnMessage OnMessage
}

var DefaultTransSrvOptions = &TransServerOption{
	MaxConnectionSize:     math.MaxUint32,
	MaxSendMessageSize:    1024 * 1024 * 4,
	MaxReceiveMessageSize: 1024 * 1024 * 4,
}
