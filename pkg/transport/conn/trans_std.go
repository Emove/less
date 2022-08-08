//go:build windows
// +build windows

package conn

import (
	"context"
	"less/internal/server"
	"less/pkg/proto"
	"less/pkg/transport"
	"less/utils/atomic"
	"net"
)

type transportServer struct {
	opts       *server.ServerOptions
	ctx        context.Context
	cancelFunc context.CancelFunc

	connCnt atomic.AtomicInt64

	proto proto.Protocol
}

var _ transport.TransportServer = (*transportServer)(nil)

func newTransportServer(ctx context.Context, opts *server.ServerOptions) transport.TransportServer {
	server := &transportServer{
		ctx:   ctx,
		opts:  opts,
		proto: &proto.GenericProtocol{},
	}
	return server
}

func (s *transportServer) Serv(network, addr string) error {
	l, err := net.Listen(network, addr)
	if err != nil {
		return err
	}
	go s.listen(l)

	return nil
}

func (s *transportServer) Stop() {

}

func (s *transportServer) listen(l net.Listener) {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := l.Accept()
			if err != nil {
				// TODO
			}
			s.onConn(conn)
		}
	}
}

func (s *transportServer) onConn(conn net.Conn) {
	if s.connCnt.Inc() > int64(s.opts.MaxConnectionSize) {
		s.connCnt.Dec()
		_ = conn.Close()
		// TODO log conn size out of limit
		return
	}

	//s. := NewConnection(conn)

}

func (s *transportServer) read() {
	//header := make([]byte, s.proto.HeaderLength())
	//if _, err := s.conn.(io.Reader).Read(header); err != nil {
	//	s.stop()
	//	return
	//}
	//body := make([]byte, int(s.proto.BodySize(header)))
	//if _, err := io.ReadFull(s.conn, body); err != nil {
	//	s.stop()
	//	return
	//}
	//// TODO
}
