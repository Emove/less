package server

import (
	"context"
	"fmt"
	"net"
	"reflect"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	engine "github.com/emove/less/internal/engine"
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
	handler    engine.TransHandler
}

type serverOptions struct {
	addr          string
	port          string
	transport     transport.Transport
	transOptions  []engine.Option
	shutdownHooks []ShutdownHook
}

// NewServer creates a server
func NewServer(addr string, op ...SerOption) *Server {
	ops := defaultServerOptions()

	for _, o := range op {
		o(ops)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	return &Server{ctx: ctx, cancelFunc: cancelFunc, addr: addr, ops: ops}
}

func defaultServerOptions() *serverOptions {
	return &serverOptions{
		addr:      "127.0.0.1",
		port:      "8888",
		transport: tcp.New(),
	}
}

// Run listens transport address and serving for channel and message request
func (srv *Server) Run() {

	srv.addr = parseAddr(srv)

	srv.handler = engine.NewEndpointHandler(srv.ctx, srv.ops.transOptions...)

	go func() {
		if err := srv.ops.transport.Listen(srv.addr, srv.handler); err != nil {
			srv.Shutdown(context.Background(), err)
		}
	}()
}

// Shutdown stops the Server, closes the transporter and all channels
func (srv *Server) Shutdown(ctx context.Context, err error) {
	// close the transportHandler to refuse connecting request and Read event
	_ = srv.handler.Close()
	hooks := srv.ops.shutdownHooks
	if len(hooks) > 0 {
		for _, hook := range hooks {
			hook(ctx, err)
		}
	}
	srv.ops.transport.Close()
	srv.cancelFunc()
	select {
	case <-srv.ctx.Done():
		return
	}
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
			ops.transOptions = append(ops.transOptions, engine.AddOnChannel(onChannel...))
		}
	}
}

// WithOnChannelClosed adds channel closed hooks
func WithOnChannelClosed(onChannelClosed ...less.OnChannelClosed) SerOption {
	return func(ops *serverOptions) {
		if len(onChannelClosed) > 0 {
			ops.transOptions = append(ops.transOptions, engine.AddOnChannelClosed(onChannelClosed...))
		}
	}
}

// WithRouter sets message router
func WithRouter(router less.Router) SerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, engine.WithRouter(router))
	}
}

// WithPacketCodec sets the packet codec used by the server transport engine.
func WithPacketCodec(c codec.PacketCodec) SerOption {
	if codecIsNil(c) {
		panic("packet codec can not be nil")
	}
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, engine.WithPacketCodec(c))
	}
}

// WithPayloadCodec sets the payload codec used by the server transport engine.
func WithPayloadCodec(c codec.PayloadCodec) SerOption {
	if codecIsNil(c) {
		panic("payload codec can not be nil")
	}
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, engine.WithPayloadCodec(c))
	}
}

// WithInboundMiddleware adds inbound middlewares
func WithInboundMiddleware(mws ...less.Middleware) SerOption {
	return func(ops *serverOptions) {
		if len(mws) > 0 {
			ops.transOptions = append(ops.transOptions, engine.AddInboundMiddleware(mws...))
		}
	}
}

// WithOutboundMiddleware adds outbound middlewares
func WithOutboundMiddleware(mws ...less.Middleware) SerOption {
	return func(ops *serverOptions) {
		if len(mws) > 0 {
			ops.transOptions = append(ops.transOptions, engine.AddOutboundMiddleware(mws...))
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
		ops.transOptions = append(ops.transOptions, engine.MaxChannelSize(size))
	}
}

// MaxSendMessageSize sets the max size of message when send
func MaxSendMessageSize(size uint32) SerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, engine.MaxSendMessageSize(size))
	}
}

// MaxReceiveMessageSize sets the max size of message when receive
func MaxReceiveMessageSize(size uint32) SerOption {
	return func(ops *serverOptions) {
		ops.transOptions = append(ops.transOptions, engine.MaxReceiveMessageSize(size))
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

func codecIsNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
