package channel

import (
	"context"
	"net"

	"github.com/emove/less/channel"
	"github.com/emove/less/internal/transport"
	"github.com/emove/less/io"
	"github.com/emove/less/middleware"
	"github.com/emove/less/transport/conn"
)

type ChannelFactory func(con conn.Connection) *Channel

type Channel struct {
	ctx    context.Context
	cancel context.CancelFunc

	conn conn.Connection

	onChannelClosed []channel.OnChannelClosed

	inboundMiddleware  middleware.Middleware
	outboundMiddleware middleware.Middleware

	h transport.ChannelHandler
}

func NewFactory(ctx context.Context, handler transport.ChannelHandler) ChannelFactory {
	return func(con conn.Connection) *Channel {
		return NewChannel(ctx, con, handler)
	}
}

func NewChannel(ctx context.Context, con conn.Connection, handler transport.ChannelHandler) *Channel {
	c, cancel := context.WithCancel(ctx)
	return &Channel{
		ctx:             c,
		cancel:          cancel,
		conn:            con,
		h:               handler,
		onChannelClosed: make([]channel.OnChannelClosed, 0),
	}
}

func (ch *Channel) SetContext(ctx context.Context) {
	ch.ctx = ctx
}

func (ch *Channel) Context() context.Context {
	return ch.ctx
}

func (ch *Channel) RemoteAddr() net.Addr {
	return ch.conn.RemoteAddr()
}

func (ch *Channel) LocalAddr() net.Addr {
	return ch.conn.LocalAddr()
}

func (ch *Channel) Reader() io.Reader {
	return ch.conn.Reader()
}

func (ch *Channel) Writer() io.Writer {
	return ch.conn.Writer()
}

func (ch *Channel) Write(msg interface{}) error {
	// TODO
	return ch.h.OnWrite(ch, ch.conn.Writer(), msg)
}

func (ch *Channel) IsActive() bool {
	return ch.conn.IsActive()
}

func (ch *Channel) Close(err error) {

	if !ch.IsActive() {
		return
	}

	ch.cancel()

	// handle channel closed event
	ch.h.OnChannelClosed(ch.ctx, ch, err, ch.onChannelClosed)

	// close connection
	_ = ch.conn.Close()
}

func (ch *Channel) AddOnChannelClosed(onChannelClosed ...channel.OnChannelClosed) {
	ch.onChannelClosed = append(ch.onChannelClosed, onChannelClosed...)
}

func (ch *Channel) AddInboundMiddleware(mw ...middleware.Middleware) {
	if len(mw) > 0 {
		ch.inboundMiddleware = middleware.Chain(ch.inboundMiddleware, middleware.Chain(mw...))
	}
}

func (ch *Channel) AddOutboundMiddleware(mw ...middleware.Middleware) {
	if len(mw) > 0 {
		ch.outboundMiddleware = middleware.Chain(ch.outboundMiddleware, middleware.Chain(mw...))
	}
}

func (ch *Channel) GetInboundMiddleware() middleware.Middleware {
	return ch.inboundMiddleware
}

func (ch *Channel) GetOutboundMiddleware() middleware.Middleware {
	return ch.outboundMiddleware
}
