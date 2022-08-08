package less

import (
	"context"
	"less/internal/server"
	"less/pkg/codec"
	transport2 "less/pkg/transport"
	"less/transport"
)

type Server struct {
	addr    string
	network string

	ctx        context.Context
	cancelFunc context.CancelFunc

	opts *server.ServerOptions

	transport transport2.TransportServer
}

func NewServer(network, addr string, ops ...ServerOption) *Server {
	opts := server.DefaultServerOptions

	for _, op := range ops {
		op.Apply(&opts)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	srv := &Server{
		ctx:        ctx,
		cancelFunc: cancelFunc,
		network:    network,
		addr:       addr,
		opts:       &opts,
	}

	srv.transport = transport.NewTransportServer(srv.ctx, srv.opts)

	return srv
}

func (srv *Server) Run() error {

	if err := srv.transport.Serv(srv.network, srv.addr); err != nil {
		return err
	}

	return nil
}

func (srv *Server) Shutdown() {
	srv.cancelFunc()
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

func MaxConnectionSize(size uint32) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.MaxConnectionSize = size
	})
}

func MaxSendMessageSize(size uint32) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.MaxSendMessageSize = size
	})
}

func MaxReceiveMessageSize(size uint32) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.MaxReceiveMessageSize = size
	})
}

func WithCodec(codec codec.Codec) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.Codec = codec
	})
}
