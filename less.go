package less

import (
	"context"
	"net"
)

type (
	// Interceptor defines the interceptor used to intercept message.
	Interceptor func(message interface{}) bool
	// Handler defines the handler invoked by Middleware.
	Handler func(ctx context.Context, ch Channel, message interface{}) error
	// Middleware is transport Middleware.
	Middleware func(handler Handler) Handler
	// OnChannel is a hook which will be invoked when received a network connect request.
	OnChannel func(ctx context.Context, ch Channel) (context.Context, error)
	// OnChannelClosed is a hook which will be invoked when channel closed.
	OnChannelClosed func(ctx context.Context, ch Channel, err error)
)

// Channel defines the public behaviors of a channel.
type Channel interface {
	// Context returns custom context if set, or returns context.Background.
	Context() context.Context

	// RemoteAddr returns the remote network address, same as net.Conn#RemoteAddr.
	RemoteAddr() net.Addr

	// LocalAddr returns the local network address, same as net.Conn#LocalAddr.
	LocalAddr() net.Addr

	// Write writes the message to channel and fires outbound middleware.
	Write(msg interface{}) error

	// IsActive returns false only when the channel closed.
	IsActive() bool

	// Close closes the channel after inbound and outbound events complete.
	Close(err error)

	// AddOnChannelClosed adds OnChannelClosed hooks for this channel.
	AddOnChannelClosed(onChannelClosed ...OnChannelClosed)

	// AddInboundMiddleware adds inbound Middleware for this channel.
	AddInboundMiddleware(mw ...Middleware)

	// AddOutboundMiddleware adds outbound Middleware for this channel.
	AddOutboundMiddleware(mw ...Middleware)
}

// Chain returns a Middleware that specifies the chained handler for transport.
func Chain(ms ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(ms) - 1; i >= 0; i-- {
			next = ms[i](next)
		}
		return next
	}
}
