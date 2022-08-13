package less

import (
	"less/internal/server"
	"less/pkg/middleware"
	"less/pkg/transport"
	"less/pkg/transport/transrv"
)

type Server struct {
	addr    string
	network string

	opts *server.ServerOptions

	transport transport.TransServer
}

func NewServer(network, addr string, ops ...ServerOption) *Server {
	opts := server.DefaultServerOptions

	for _, op := range ops {
		op.Apply(&opts)
	}

	srv := &Server{
		network: network,
		addr:    addr,
		opts:    &opts,
	}

	// pass in msgHandler
	srv.transport = transrv.NewTransportServer(srv.opts.TransSrvOps, nil)

	return srv
}

func (srv *Server) Run() error {

	if err := srv.transport.Serv(srv.network, srv.addr); err != nil {
		return err
	}

	return nil
}

func (srv *Server) Shutdown() {
	srv.transport.Stop()
}

type ServerOption interface {
	Apply(*server.ServerOptions)
}

type funcServerOption struct {
	f func(options *server.ServerOptions)
}

func newFuncServerOption(f func(options *server.ServerOptions)) ServerOption {
	return &funcServerOption{
		f: f,
	}
}

func (fso *funcServerOption) Apply(so *server.ServerOptions) {
	fso.f(so)
}

func WithOnMessage(onMessage transport.OnMessage) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.TransSrvOps.OnMessage = onMessage
	})
}

func AddInboundMiddleware(mdw middleware.Handler) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.MsgHandlerOps.InboundHandlers = append(options.MsgHandlerOps.InboundHandlers, mdw)
	})
}

func AddOutboundMiddleware(mdw middleware.Handler) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.MsgHandlerOps.OutboundHandlers = append(options.MsgHandlerOps.OutboundHandlers, mdw)
	})
}

func MaxConnectionSize(size uint32) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.TransSrvOps.MaxConnectionSize = size
	})
}

func MaxSendMessageSize(size uint32) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.TransSrvOps.MaxSendMessageSize = size
	})
}

func MaxReceiveMessageSize(size uint32) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.TransSrvOps.MaxReceiveMessageSize = size
	})
}

func WithCodec(codec transport.Codec) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.TransSrvOps.Codec = codec
	})
}
