package less

import (
	"context"
	"net"
)

type (
	OnChannel func(ctx context.Context, ch Channel) (context.Context, error)

	OnChannelClosed func(ctx context.Context, ch Channel, err error)
)

type Channel interface {
	Context() context.Context

	RemoteAddr() net.Addr

	LocalAddr() net.Addr

	Write(msg interface{}) error

	IsActive() bool

	Close(err error)

	// CloseReader()

	// CloseWriter()

	AddOnChannelClosed(onChannelClosed ...OnChannelClosed)

	AddInboundMiddleware(mw ...Middleware)

	AddOutboundMiddleware(mw ...Middleware)
}
