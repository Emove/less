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

// NewTransport defines a func to new transport.
type NewTransport func(ctx context.Context, ops ...Option) Transport

// Listener defines the behaviors of Listener.
type Listener interface {
	// Listen the addr and accept the network connection request.
	Listen(addr string, driver EventDriver) error
	// Close closes the Listener.
	Close()
}

// Dialer defines a Dialer.
type Dialer interface {
	// Dial dials the remote endpoint.
	Dial(net, addr string, driver EventDriver) error
}

// Transport defines a Transport
type Transport interface {
	Listener
	Dialer
}
