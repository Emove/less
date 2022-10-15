package tcp

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/emove/less/internal/recovery"
	"github.com/emove/less/log"
	trans "github.com/emove/less/transport"
)

type transport struct {
	ctx    context.Context
	cancel context.CancelFunc
	ops    *TCPOptions
}

var _ trans.Transport = (*transport)(nil)

func New(op ...trans.Option) trans.Transport {

	ops := DefaultOptions
	for _, o := range op {
		o(ops)
	}

	return &transport{
		ops: ops,
	}
}

func (t *transport) Listen(addr string, driver trans.EventDriver) error {
	tcpAddr, err := net.ResolveTCPAddr(t.ops.Network, addr)
	if err != nil {
		return err
	}
	listener, err := net.ListenTCP(tcpAddr.Network(), tcpAddr)
	if err != nil {
		return err
	}

	log.Infof(fmt.Sprintf("transport listening, network: %s, address: %s", t.ops.Network, addr))

	t.ctx, t.cancel = context.WithCancel(context.Background())

	var con net.Conn
	for {
		con, err = listener.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Errorf("tcp accept err: %v, retrying in 200 ms", err)
				time.Sleep(200 * time.Millisecond)
				err = nil
			} else {
				return err
			}
		}
		tc := con.(*net.TCPConn)

		if err = t.applyOptions(tc, t.ops); err != nil {
			log.Errorf("config tcp connection err: %v", err)
			_ = tc.Close()
			continue
		}

		cc := context.Background()
		wrapped := WrapConnection(con)
		cc, err = driver.OnConnect(cc, wrapped)
		if err != nil {
			continue
		}

		go t.readLoop(cc, wrapped, driver)
	}
}

func (t *transport) Dial(network, addr string, driver trans.EventDriver) error {
	remoteAddr, err := net.ResolveTCPAddr(network, addr)
	if err != nil {
		return err
	}

	var con net.Conn
	if t.ops.Timeout > 0 {
		if con, err = net.DialTimeout(remoteAddr.Network(), remoteAddr.String(), t.ops.Timeout); err != nil {
			return err
		}
	} else {
		if con, err = net.Dial(remoteAddr.Network(), remoteAddr.String()); err != nil {
			return err
		}
	}

	if err = t.applyOptions(con.(*net.TCPConn), t.ops); err != nil {
		_ = con.Close()
		return err
	}

	cc := context.Background()
	wrapped := WrapConnection(con)
	if cc, err = driver.OnConnect(cc, wrapped); err != nil {
		_ = con.Close()
	}

	go t.readLoop(cc, wrapped, driver)
	return nil
}

func (t *transport) Close() {
	if t.cancel != nil {
		t.cancel()
	}
}

func (t *transport) readLoop(ctx context.Context, conn trans.Connection, driver trans.EventDriver) {
	recovery.Recover(func(err error) {
		// trigger onConnClosed event
		driver.OnConnClosed(ctx, conn, err)
	})

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			_ = driver.OnMessage(ctx, conn)
		}
	}
}

func (t *transport) applyOptions(con *net.TCPConn, ops *TCPOptions) error {

	if err := con.SetKeepAlive(ops.Keepalive); nil != err {
		return err
	}

	if err := con.SetKeepAlivePeriod(ops.KeepAlivePeriod); nil != err {
		return err
	}

	if err := con.SetLinger(ops.Linger); nil != err {
		return err
	}

	if err := con.SetNoDelay(ops.NoDelay); nil != err {
		return err
	}

	return nil
}
