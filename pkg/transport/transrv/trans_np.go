//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package transrv

import (
	"context"
	"github.com/cloudwego/netpoll"
	"less/pkg/transport"
	"less/pkg/transport/conn"
	"less/pkg/transport/trans"
	"less/utils/atomic"
)

type transportServer struct {
	ctx context.Context

	opts *transport.TransServerOption

	listener  netpoll.Listener
	eventLoop netpoll.EventLoop

	connCnt atomic.AtomicInt64
}

func NewTransportServer(ctx context.Context, opts *transport.TransServerOption) transport.TransServer {
	return &transportServer{
		ctx:  ctx,
		opts: opts,
	}
}

func (s *transportServer) Serv(network, addr string) error {
	ops := []netpoll.Option{
		netpoll.WithReadTimeout(s.opts.ReadTimeout),
		netpoll.WithOnConnect(s.onConn),
	}

	loop, err := netpoll.NewEventLoop(s.onRequest, ops...)
	if err != nil {
		return err
	}
	s.eventLoop = loop

	// use netpoll.Listener so that closing it also
	// stops the event loop in netpoll
	listen, err := netpoll.CreateListener(network, addr)
	if err != nil {
		return err
	}
	s.listener = listen

	return s.eventLoop.Serve(listen)
}

func (s *transportServer) Stop() {
	err := s.listener.Close()
	if err != nil {
		// TODO
	}

	err = s.eventLoop.Shutdown(context.TODO())
	if err != nil {
		// TODO
	}
}

func (s *transportServer) onRequest(ctx context.Context, con netpoll.Connection) error {
	err := trans.NewTransportHandler(s.opts, conn.WrapConnection(con)).OnRequest()
	if err != nil {
		_ = con.Close()
	}
	return nil
}

func (s *transportServer) onConn(ctx context.Context, con netpoll.Connection) context.Context {
	s.connCnt.Inc()
	_ = con.AddCloseCallback(func(connection netpoll.Connection) error {
		s.opts.OnConnClose(conn.WrapConnection(con))
		s.connCnt.Dec()
		return nil
	})
	return ctx
}
