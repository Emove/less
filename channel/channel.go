package channel

import (
	"context"
	"net"

	"github.com/emove/less/middleware"
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

	AddInboundMiddleware(mw ...middleware.Middleware)

	AddOutboundMiddleware(mw ...middleware.Middleware)
}

type ctxChannelKey struct{}

func NewChannelContext(ctx context.Context, ch Channel) context.Context {
	return context.WithValue(ctx, ctxChannelKey{}, ch)
}

func FromChannelContext(ctx context.Context) (ch Channel, ok bool) {
	ch, ok = ctx.Value(ctxChannelKey{}).(Channel)
	return
}
