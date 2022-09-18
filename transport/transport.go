package transport

import (
	"context"
	"github.com/emove/less/transport/conn"
)

// EventDriver defines some func to provide cut point around a connection lifecycle
type EventDriver interface {
	// OnConnect fires when receive a connect request
	// it supports to do something like check the connection before connection active
	// the returned context will pass though all of else events as the first parameter
	// if the error returned, the connection will be rejected
	OnConnect(ctx context.Context, con conn.Connection) (context.Context, error)
	// OnMessage fires when receive a request
	OnMessage(ctx context.Context, con conn.Connection) error
	// OnConnClosed should be called when the connection be closed
	OnConnClosed(ctx context.Context, con conn.Connection, err error)
}

type NewTransport func(ctx context.Context, ops ...Option) Transport

type Listener interface {
	Listen(addr string, driver EventDriver) error
	Close()
}

type Dialer interface {
	Dial(net, addr string, driver EventDriver) error
}

type Transport interface {
	Listener
	Dialer
}
