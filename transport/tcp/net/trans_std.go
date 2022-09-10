package net

import (
	"context"
	"github.com/emove/less/internal/errors"
	trans "github.com/emove/less/transport"
	"github.com/emove/less/transport/conn"
	"github.com/emove/less/transport/tcp"
	"net"
)

type transport struct {
	ctx         context.Context
	cancel      context.CancelFunc
	eventDriver trans.EventDriver
}

var _ trans.Transport = (*transport)(nil)

func New(ctx context.Context, driver trans.EventDriver) trans.Transport {
	return &transport{
		ctx:         ctx,
		eventDriver: driver,
	}
}

func (t *transport) Listen(addr net.Addr, ops trans.Options) error {
	tcpAddr, err := net.ResolveTCPAddr(addr.Network(), addr.String())
	if err != nil {
		return err
	}
	listener, err := net.ListenTCP(tcpAddr.Network(), tcpAddr)
	if err != nil {
		return err
	}

	t.ctx, t.cancel = context.WithCancel(t.ctx)

	driver := t.eventDriver

	for {
		select {
		case <-t.ctx.Done():
			return nil
		default:
			con, err := listener.Accept()
			if err != nil {
				return err
			}
			tc := con.(*net.TCPConn)

			if err = t.applyOptions(tc, ops.NetOption.(*tcp.TCPOptions)); err != nil {
				// TODO handle err
				_ = tc.Close()
				continue
			}

			cc := context.Background()
			wrapped := WrapConnection(con)
			cc, err = driver.OnConnect(cc, wrapped)
			if err != nil {
				driver.OnConnClosed(cc, wrapped, err)
				_ = con.Close()
				continue
			}

			go t.readLoop(cc, wrapped)
		}
	}
}

func (t *transport) Dial(addr net.Addr, ops trans.Options) error {
	to := ops.NetOption.(*tcp.TCPOptions)
	localAddr, err := net.ResolveTCPAddr(addr.Network(), addr.String())
	if err != nil {
		return err
	}
	remoteAddr, err := net.ResolveTCPAddr(to.Remote.Network(), to.Remote.String())
	if err != nil {
		return err
	}

	// TODO use net.DialTimeout

	tc, err := net.DialTCP(localAddr.Network(), localAddr, remoteAddr)
	if err != nil {
		return err
	}

	if err = t.applyOptions(tc, ops.NetOption.(*tcp.TCPOptions)); err != nil {
		_ = tc.Close()
		return err
	}

	cc := context.Background()
	wrapped := WrapConnection(tc)
	cc, err = t.eventDriver.OnConnect(cc, wrapped)
	if err != nil {
		_ = tc.Close()
	}

	go t.readLoop(cc, wrapped)
	return nil
}

func (t *transport) Close() {
	if t.cancel != nil {
		t.cancel()
	}
}

func (t *transport) readLoop(ctx context.Context, conn conn.Connection) {

	driver := t.eventDriver
	var err error
	defer func() {
		if e := recover(); e != nil {
			err = errors.AsError(e)
		}

		// trigger onConnClosed event
		driver.OnConnClosed(ctx, conn, err)
	}()

	for {
		select {
		case <-t.ctx.Done():
			return
		default:
			if err = driver.OnRequest(ctx, conn); err != nil {
				// TODO log
				return
			}
		}
	}
}

func (t *transport) applyOptions(con *net.TCPConn, ops *tcp.TCPOptions) error {

	if err := con.SetKeepAlive(ops.KeepAlive); nil != err {
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
