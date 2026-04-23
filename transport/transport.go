package transport

import (
	"context"
)

// EventDriver defines some func to provide cut point around a connection lifecycle.
type EventDriver interface {
	// OnConnect fires when receive a connect request.
	// it supports to do something like check the connection before connection active
	// the returned context will pass though all of else events as the first parameter
	// if the error returned, the connection will be rejected.
	OnConnect(ctx context.Context, con Connection) (context.Context, error)
	// OnMessage fires when receive a request.
	OnMessage(ctx context.Context, con Connection) error
	// OnConnClosed should be called when the connection be closed.
	OnConnClosed(ctx context.Context, con Connection, err error)
}

// Transport defines a Transport
type Transport interface {
	// Listen listens on the given address and uses the driver for connection events.
	Listen(addr string, driver EventDriver) error
	// Dial dials the remote endpoint and uses the driver for connection events.
	Dial(network, addr string, driver EventDriver) error
	// Close closes the Transport.
	Close()
}
