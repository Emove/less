//go:build windows
// +build windows

package transrv

import (
	"context"
	"net"

	"less/internal/handler"
	"less/pkg/proto"
	"less/pkg/transport"
	"less/pkg/transport/conn"
	"less/pkg/transport/trans"
	"less/utils/atomic"
)

type transportServer struct {
	opts       *transport.TransServerOption
	ctx        context.Context
	cancelFunc context.CancelFunc

	pipeline *handler.PipelineHandler

	connCnt atomic.AtomicInt64
}

var _ transport.TransServer = (*transportServer)(nil)

func NewTransportServer(opts *transport.TransServerOption, msgHandler *handler.PipelineHandler) transport.TransServer {
	srv := &transportServer{
		opts:     opts,
		pipeline: msgHandler,
	}
	return srv
}

func (s *transportServer) Serv(network, addr string) error {
	s.ctx, s.cancelFunc = context.WithCancel(context.Background())

	l, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	go s.listen(l)

	return nil
}

func (s *transportServer) Stop() {
	// stop all sub goroutine
	s.cancelFunc()
}

func (s *transportServer) listen(l net.Listener) {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			con, err := l.Accept()
			if err != nil {
				// TODO
			}
			s.onConn(con)
		}
	}
}

func (s *transportServer) onConn(con net.Conn) {
	if s.connCnt.Inc() > int64(s.opts.MaxConnectionSize) {
		s.connCnt.Dec()
		_ = con.Close()
		// TODO log conn size out of limit
		return
	}

	proxy := &conn.ConProxy{
		Ctx:       s.ctx,
		Raw:       con,
		HeaderLen: proto.GenericProtocol.HeaderLength(),
		OnConnClose: func(conn transport.Connection, err error) {
			s.closeConn(conn, err)
		},
	}
	transConn := conn.WrapConnection(proxy)

	// call custom onConn hook
	s.opts.OnConn(transConn)

	// set read timeout
	_ = transConn.SetReadTimeout(s.opts.ReadTimeout)

	// new transport handler
	transHandler := trans.NewTransportHandler(s.opts, transConn, s.pipeline)

	proxy.Conn = transConn
	proxy.OnRequest = func(conn transport.Connection) error {
		return transHandler.OnRequest()
	}

	// read data
	go proxy.ReadLoop()
}

func (s *transportServer) closeConn(conn transport.Connection, err error) {
	// TODO handle err

	// call OnConnClose hook
	s.opts.OnConnClose(conn)

	// close connection and dec connection count
	_ = conn.Close()
	s.connCnt.Dec()
}
