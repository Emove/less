package less

import (
	"context"
	"net"
)

type (
	// OnChannel is a hook which will be invoked when received a network connect request.
	OnChannel func(ctx context.Context, ch Channel) (context.Context, error)
	// OnChannelClosed is a hook which will be invoked when channel closed.
	OnChannelClosed func(ctx context.Context, ch Channel, err error)
)

// Channel defines the behaviors of channel.
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

	// Close closing the channel after inbound and outbound event done.
	Close(ctx context.Context, err error) error

	// CloseReader closes the channel reader then the channel is unable to receive any message.
	CloseReader()

	// CloseWriter close the channel writer then the channel is unable to send any message.
	CloseWriter()

	// Readable returns the channel readable or not
	Readable() bool

	// Writeable returns the channel writeable or not
	Writeable() bool

	// AddOnChannelClosed adds OnChannelClosed hooks for this channel.
	AddOnChannelClosed(onChannelClosed ...OnChannelClosed)

	// AddInboundMiddleware adds inbound Middleware for this channel.
	AddInboundMiddleware(mw ...Middleware)

	// AddOutboundMiddleware adds outbound Middleware for this channel.
	AddOutboundMiddleware(mw ...Middleware)
}
