package transport

import (
	"net"
)

// TransServer is an abstraction for net on different platform
type TransServer interface {
	Serv(network, addr string) error
	Stop()
}

type TransHandler interface {
	OnRequest() error
}

// Transporter is an abstraction for send data
type Transporter interface {
	Remote() net.Addr
	Local() net.Addr
	Send(data interface{}) error
}
