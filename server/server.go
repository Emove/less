package server

import (
	"context"
	"fmt"
	"github.com/emove/less"
	inter_server "github.com/emove/less/internal/server"
	"github.com/emove/less/internal/transport"
	"github.com/emove/less/pkg/router"
	trans "github.com/emove/less/transport"
	"net"
	"time"
)

type Server struct {
	addr string
	ops  *inter_server.Options

	handler transport.TransHandler
}

func NewServer(addr string, op ...Option) *Server {
	ops := inter_server.DefaultServerOptions

	for _, o := range op {
		o.Apply(ops)
	}

	return &Server{addr: addr, ops: ops}
}

func (srv *Server) Run() {

	srv.addr = parseAddr(srv)

	srv.handler = transport.NewTransHandler(srv.ops.TransOptions...)

	go func() {
		err := srv.ops.Transport.Listen(srv.addr, srv.handler)
		if err != nil {
			srv.Shutdown()
		}
	}()
}

func (srv *Server) Shutdown() {
	_ = srv.handler.Close(context.Background(), nil)
	srv.ops.Transport.Close()
}

type Option interface {
	Apply(*inter_server.Options)
}

type serverOption struct {
	f func(options *inter_server.Options)
}

func newOption(f func(ops *inter_server.Options)) Option {
	return &serverOption{
		f: f,
	}
}

func (fso *serverOption) Apply(so *inter_server.Options) {
	fso.f(so)
}

func WithTransport(transport trans.Transport) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.Transport = transport
	})
}

func WithOnChannel(onChannel ...less.OnChannel) Option {
	return newOption(func(ops *inter_server.Options) {
		if len(onChannel) > 0 {
			ops.TransOptions = append(ops.TransOptions, transport.OnChannel(onChannel...))
		}
	})
}

func WithOnChannelClosed(onChannelClosed ...less.OnChannelClosed) Option {
	return newOption(func(ops *inter_server.Options) {
		if len(onChannelClosed) > 0 {
			ops.TransOptions = append(ops.TransOptions, transport.OnChannelClosed(onChannelClosed...))
		}
	})
}

func WithRouter(router router.Router) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithRouter(router))
	})
}

func WithInboundMiddleware(mws ...less.Middleware) Option {
	return newOption(func(ops *inter_server.Options) {
		if len(mws) > 0 {
			ops.TransOptions = append(ops.TransOptions, transport.WithInbound(mws...))
		}
	})
}

func WithOutboundMiddleware(mws ...less.Middleware) Option {
	return newOption(func(ops *inter_server.Options) {
		if len(mws) > 0 {
			ops.TransOptions = append(ops.TransOptions, transport.WithOutbound(mws...))
		}
	})
}

func MaxIdleTime(d time.Duration) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithIdleTime(d))
	})
}

func MaxConnectionSize(size uint32) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithMaxConnectionSize(size))
	})
}

func MaxSendMessageSize(size uint32) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithMaxSendMessageSize(size))
	})
}

func MaxReceiveMessageSize(size uint32) Option {
	return newOption(func(ops *inter_server.Options) {
		ops.TransOptions = append(ops.TransOptions, transport.WithMaxReceiveMessageSize(size))
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
