package server

import (
	"context"
	"fmt"
	"net"

	"github.com/emove/less"
	trans "github.com/emove/less/internal/transport"
	"github.com/emove/less/router"
	"github.com/emove/less/transport"
	"github.com/emove/less/transport/tcp"
)

type (
	ShutdownHook func(ctx context.Context, err error)
)

// Server is a network server
type Server struct {
	addr       string
	ctx        context.Context
	cancelFunc context.CancelFunc
	ops        *serverOptions
	handler    trans.TransHandler
}

var defaultServerOptions = &serverOptions{
	addr:      "127.0.0.1",
	port:      "8888",
	transport: tcp.New(),
}

type serverOptions struct {
	addr          string
	port          string
	transport     transport.Transport
	transOptions  []trans.Option
	shutdownHooks []ShutdownHook
}

// NewServer creates a less server
func NewServer(addr string, op ...SerOption) *Server {
	ops := defaultServerOptions

	for _, o := range op {
		o(ops)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	return &Server{ctx: ctx, cancelFunc: cancelFunc, addr: addr, ops: ops}
}

// Run listens transport address and serving for channel and message request
func (srv *Server) Run() {

	srv.addr = parseAddr(srv)

	srv.handler = trans.NewTransHandler(srv.ops.transOptions...)

	go func() {
		switch srv.ops.transport.(type) {
		case transport.DefaultTransport:
			//_transport := srv.ops.transport.(transport.DefaultTransport)
			//err := _transport.Listen(srv.addr, srv.handler)
			//if err != nil {
			//	srv.Shutdown()
			//	log.Fatalf("less exits because err: %v", err)
			//}
		}

	}()
}

// Shutdown stops the Server, closes the transporter and all channels
func (srv *Server) Shutdown(ctx context.Context, err error) {
	// close the transportHandler to refuse new connection request and Read event
	_ = srv.handler.Close(context.Background(), err)
	hooks := srv.ops.shutdownHooks
	if len(hooks) > 0 {
		for _, hook := range hooks {
			hook(ctx, err)
		}
	}
	_ = srv.ops.transport.Close(context.Background(), err)
}

type SerOption func(options *serverOptions)

// WithTransport sets transporter
func WithTransport(transport transport.Transport) SerOption {
	return func(ops *serverOptions) {
		ops.transport = transport
	}
}

// WithOnChannel adds channel request hooks
func WithOnChannel(onChannel ...less.OnChannel) SerOption {
	return func(ops *serverOptions) {
		if len(onChannel) > 0 {
			ops.transOptions = append(ops.transOptions, trans.AddOnChannel(onChannel...))
		}
	}
}

// WithOnChannelClosed adds channel closed hooks
func WithOnChannelClosed(onChannelClosed ...less.OnChannelClosed) SerOption {
	return func(ops *serverOptions) {
		if len(onChannelClosed) > 0 {
			ops.transOptions = append(ops.transOptions, trans.AddOnChannelClosed(onChannelClosed...))
		}
	}
}

// WithRouter sets message router
func WithRouter(router router.Router) SerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, trans.WithRouter(router))
	}
}

// WithInboundMiddleware adds inbound middlewares
func WithInboundMiddleware(mws ...less.Middleware) SerOption {
	return func(ops *serverOptions) {
		if len(mws) > 0 {
			ops.transOptions = append(ops.transOptions, trans.AddInboundMiddleware(mws...))
		}
	}
}

// WithOutboundMiddleware adds outbound middlewares
func WithOutboundMiddleware(mws ...less.Middleware) SerOption {
	return func(ops *serverOptions) {
		if len(mws) > 0 {
			ops.transOptions = append(ops.transOptions, trans.AddOutboundMiddleware(mws...))
		}
	}
}

func WithShutdownHooks(hooks ...ShutdownHook) SerOption {
	return func(options *serverOptions) {
		if len(hooks) > 0 {
			options.shutdownHooks = append(options.shutdownHooks, hooks...)
		}
	}
}

// MaxChannelSize sets the max size of channels
func MaxChannelSize(size uint32) SerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, trans.MaxChannelSize(size))
	}
}

// MaxSendMessageSize sets the max size of message when send
func MaxSendMessageSize(size uint32) SerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, trans.MaxSendMessageSize(size))
	}
}

// MaxReceiveMessageSize sets the max size of message when receive
func MaxReceiveMessageSize(size uint32) SerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, trans.MaxReceiveMessageSize(size))
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
