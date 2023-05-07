package server

import (
	"context"
	"fmt"
	"github.com/emove/less/log"
	"net"

	"github.com/emove/less"
	"github.com/emove/less/internal/transport"
	"github.com/emove/less/router"
	"github.com/emove/less/transport"
	"github.com/emove/less/transport/tcp"
)

// Server is a network server
type Server struct {
	addr string
	ops  *serverOptions

	handler transport.TransHandler
}

var defaultServerOptions = &serverOptions{
	addr:      "127.0.0.1",
	port:      "8888",
	transport: tcp.New(),
}

type serverOptions struct {
	addr         string
	port         string
	transport    transport.Transport
	transOptions []transport.Option
}

// NewServer creates a less server
func NewServer(addr string, op ...ServerOption) *Server {
	ops := defaultServerOptions

	for _, o := range op {
		o(ops)
	}

	return &Server{addr: addr, ops: ops}
}

// Run listens transport address and serving for channel and message request
func (srv *Server) Run() {

	srv.addr = parseAddr(srv)

	srv.handler = transport.NewTransHandler(srv.ops.transOptions...)

	go func() {
		err := srv.ops.transport.Listen(srv.addr, srv.handler)
		if err != nil {
			srv.Shutdown()
			log.Fatalf("less exits because err: %v", err)
		}
	}()
}

// Shutdown stops the Server, closes the transporter and all channels
func (srv *Server) Shutdown() {
	_ = srv.handler.Close(context.Background(), nil)
	srv.ops.transport.Close()
}

type ServerOption func(options *serverOptions)

// WithTransport sets transporter
func WithTransport(transport transport.Transport) ServerOption {
	return func(ops *serverOptions) {
		ops.transport = transport
	}
}

// WithOnChannel adds channel request hooks
func WithOnChannel(onChannel ...less.OnChannel) ServerOption {
	return func(ops *serverOptions) {
		if len(onChannel) > 0 {
			ops.transOptions = append(ops.transOptions, transport.AddOnChannel(onChannel...))
		}
	}
}

// WithOnChannelClosed adds channel closed hooks
func WithOnChannelClosed(onChannelClosed ...less.OnChannelClosed) ServerOption {
	return func(ops *serverOptions) {
		if len(onChannelClosed) > 0 {
			ops.transOptions = append(ops.transOptions, transport.AddOnChannelClosed(onChannelClosed...))
		}
	}
}

// WithRouter sets message router
func WithRouter(router router.Router) ServerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, transport.WithRouter(router))
	}
}

// WithInboundMiddleware adds inbound middlewares
func WithInboundMiddleware(mws ...less.Middleware) ServerOption {
	return func(ops *serverOptions) {
		if len(mws) > 0 {
			ops.transOptions = append(ops.transOptions, transport.AddInboundMiddleware(mws...))
		}
	}
}

// WithOutboundMiddleware adds outbound middlewares
func WithOutboundMiddleware(mws ...less.Middleware) ServerOption {
	return func(ops *serverOptions) {
		if len(mws) > 0 {
			ops.transOptions = append(ops.transOptions, transport.AddOutboundMiddleware(mws...))
		}
	}
}

// MaxChannelSize sets the max size of channels
func MaxChannelSize(size uint32) ServerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, transport.MaxChannelSize(size))
	}
}

// MaxSendMessageSize sets the max size of message when send
func MaxSendMessageSize(size uint32) ServerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, transport.MaxSendMessageSize(size))
	}
}

// MaxReceiveMessageSize sets the max size of message when receive
func MaxReceiveMessageSize(size uint32) ServerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, transport.MaxReceiveMessageSize(size))
	}
}

func parseAddr(srv *Server) string {
	addr, port, _ := net.SplitHostPort(srv.addr)

	if len(addr) == 0 {
		addr = srv.ops.addr
	}

	if len(port) == 0 {
		port = srv.ops.port
	}

	return fmt.Sprintf("%s:%s", addr, port)
}
