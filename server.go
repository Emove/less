package less

import (
	"context"
	"fmt"
	"net"

	inter_server "github.com/emove/less/internal/server"
	"github.com/emove/less/internal/transport"
	"github.com/emove/less/keepalive"
	_go "github.com/emove/less/pkg/pool/go"
	"github.com/emove/less/router"
	trans "github.com/emove/less/transport"
)

// Server is a network server
type Server struct {
	addr string
	ops  *inter_server.Options

	handler transport.TransHandler
}

// NewServer creates a less server
func NewServer(addr string, op ...Option) *Server {
	ops := inter_server.DefaultServerOptions

	for _, o := range op {
		o.apply(ops)
	}

	return &Server{addr: addr, ops: ops}
}

// Run listens transport address and serving for channel and message request
func (srv *Server) Run() {

	srv.addr = parseAddr(srv)

	srv.handler = transport.NewTransHandler(srv.ops.TransOptions...)

	if !srv.ops.DisableGPool {
		_go.Init()
	}

	go func() {
		err := srv.ops.Transport.Listen(srv.addr, srv.handler)
		if err != nil {
			srv.Shutdown()
		}
	}()
}

// Shutdown stops the Server, closes the transporter and all channels
func (srv *Server) Shutdown() {
	_ = srv.handler.Close(context.Background(), nil)
	srv.ops.Transport.Close()
	_go.Release()
}

type Option interface {
	apply(*inter_server.Options)
}

type serverOption struct {
	f func(options *inter_server.Options)
}

func newOption(f func(ops *inter_server.Options)) Option {
	return &serverOption{
		f: f,
	}
}

func (fso *serverOption) apply(so *inter_server.Options) {
	fso.f(so)
}

// WithTransport sets transporter
func WithTransport(transport trans.Transport) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.Transport = transport
	})
}

// WithOnChannel adds channel request hooks
func WithOnChannel(onChannel ...OnChannel) Option {
	return newOption(func(ops *inter_server.Options) {
		if len(onChannel) > 0 {
			ops.TransOptions = append(ops.TransOptions, transport.OnChannel(onChannel...))
		}
	})
}

// WithOnChannelClosed adds channel closed hooks
func WithOnChannelClosed(onChannelClosed ...OnChannelClosed) Option {
	return newOption(func(ops *inter_server.Options) {
		if len(onChannelClosed) > 0 {
			ops.TransOptions = append(ops.TransOptions, transport.OnChannelClosed(onChannelClosed...))
		}
	})
}

// WithRouter sets message router
func WithRouter(router router.Router) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithRouter(router))
	})
}

// KeepaliveParams sets keepalive parameters
func KeepaliveParams(kp keepalive.KeepaliveParameters) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.Keepalive(kp))
	})
}

// WithInboundMiddleware adds inbound middlewares
func WithInboundMiddleware(mws ...Middleware) Option {
	return newOption(func(ops *inter_server.Options) {
		if len(mws) > 0 {
			ops.TransOptions = append(ops.TransOptions, transport.WithInbound(mws...))
		}
	})
}

// WithOutboundMiddleware adds outbound middlewares
func WithOutboundMiddleware(mws ...Middleware) Option {
	return newOption(func(ops *inter_server.Options) {
		if len(mws) > 0 {
			ops.TransOptions = append(ops.TransOptions, transport.WithOutbound(mws...))
		}
	})
}

// MaxChannelSize sets the max size of channels
func MaxChannelSize(size uint32) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithMaxChannelSize(size))
	})
}

// MaxSendMessageSize sets the max size of message when send
func MaxSendMessageSize(size uint32) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithMaxSendMessageSize(size))
	})
}

// MaxReceiveMessageSize sets the max size of message when receive
func MaxReceiveMessageSize(size uint32) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithMaxReceiveMessageSize(size))
	})
}

// DisableGoPool disables ants goroutine pool
func DisableGoPool() Option {
	return newOption(func(ops *inter_server.Options) {
		ops.DisableGPool = true
	})
}

// MaxGoPoolCapacity sets the max size of ants goroutine pool
func MaxGoPoolCapacity(size int) Option {
	return newOption(func(ops *inter_server.Options) {
		_go.DefaultAntsPoolSize = size
	})
}

func parseAddr(srv *Server) string {
	addr, port, _ := net.SplitHostPort(srv.addr)

	if len(addr) == 0 {
		addr = srv.ops.Addr
	}

	if len(port) == 0 {
		port = srv.ops.Port
	}

	return fmt.Sprintf("%s:%s", addr, port)
}
