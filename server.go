package less

import (
	"less/internal/server"
	"less/proto"
	"less/transport"
)

type Server struct {
	addr    string
	network string

	opts server.ServerOptions

	transport *transport.TransportServer
}

func NewServer(network, addr string, ops ...ServerOption) *Server {
	opts := server.DefaultServerOptions

	for _, op := range ops {
		op.Apply(&opts)
	}

	srv := &Server{
		network: network,
		addr:    addr,
		opts:    opts,
	}

	return srv
}

func (srv *Server) Run() {
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

func WithCodec(codec proto.Codec) ServerOption {
	return newFuncServerOption(func(options *server.ServerOptions) {
		options.Codec = codec
	})
}
