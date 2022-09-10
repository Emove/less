//go:build darwin || netbsd || freebsd || openbsd || dragonfly || linux
// +build darwin netbsd freebsd openbsd dragonfly linux

package netpoll

import (
	"context"
	"fmt"
	inter_ch "github.com/emove/less/internal/channel"
	trs "github.com/emove/less/internal/transport"
	"github.com/emove/less/transport/tcp"

	"github.com/cloudwego/netpoll"
)

type ctxNetpollCon struct{}

type transport struct {
	ctx context.Context
	ops trs.Options

	listener netpoll.Listener
	el       netpoll.EventLoop

	onConnectHandler trs.OnConnect
	onRequestHandler trs.OnRequest
}

func NewTransport(ctx context.Context, onConnect trs.OnConnect, onRequest trs.OnRequest) trs.Transport {
	return &transport{
		ctx:              ctx,
		onConnectHandler: onConnect,
		onRequestHandler: onRequest,
	}
}

func (t *transport) Listen(ops trs.Options) error {

	t.ops = ops

	to := ops.NetOption.(*tcp.TCPOptions)

	npOps := []netpoll.Option{
		netpoll.WithReadTimeout(to.ReadTimeout),
		netpoll.WithOnConnect(t.onConn),
	}

	el, err := netpoll.NewEventLoop(t.onRequest, npOps...)
	if err != nil {
		return err
	}
	t.el = el

	// use netpoll.Listener so that closing it also
	// stops the event loop in netpoll
	listen, err := netpoll.CreateListener(ops.Addr.Network(), ops.Addr.String())
	if err != nil {
		return err
	}
	t.listener = listen

	return el.Serve(listen)

}

func (t *transport) Dial(ctx context.Context, ops trs.Options) error {
	to := ops.NetOption.(*tcp.TCPOptions)
	local := ops.Addr
	localAddr, err := netpoll.ResolveTCPAddr(local.Network(), local.String())
	if err != nil {
		return err
	}
	remoteAddr, err := netpoll.ResolveTCPAddr(to.Remote.Network(), to.Remote.String())
	if err != nil {
		return err
	}
	tc, err := netpoll.DialTCP(ctx, localAddr.Network(), localAddr, remoteAddr)
	if err != nil {
		return err
	}

	// TODO
	if err = tc.SetOnRequest(t.onRequest); err != nil {
		return err
	}
	tc.Fd()

	con := WrapConnection(tc)
	ch := t.onConnectHandler(ctx, con)

	go func() {
		for {
			select {
			case <-ch.Context().Done():
				return
			default:
				t.onRequestHandler(ch)
			}
		}
	}()
	//ch := inter_ch.NewChannel(wrapped, t.middleware)
	//ctx = t.middleware.OnConnect(ctx, ch)
	//ch.SetContext(ctx)
	return nil
}

func (t *transport) Close() {
	_ = t.el.Shutdown(t.ctx)
}

func (t *transport) onRequest(ctx context.Context, _ netpoll.Connection) error {
	ch := ctx.Value(ctxNetpollCon{}).(*inter_ch.Channel)

	err := t.handler.OnRead(ch)
	if err != nil {
		ch.Close(err)
		return err
	}

	return nil
}

func (t *transport) onConn(ctx context.Context, con netpoll.Connection) context.Context {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case error:
			default:
				err = fmt.Errorf("%v", err)
			}
		}

		_ = con.Close()
	}()

	// set read timeout
	to := t.ops.NetOption.(*tcp.TCPOptions)
	_ = con.SetReadTimeout(to.ReadTimeout)

	selfCtx := context.Background()
	wrapped := WrapConnection(con)
	ch := inter_ch.NewChannel(wrapped, t.handler)

	// call OnConnect
	selfCtx = t.handler.OnConnect(selfCtx, ch)
	ch.SetContext(selfCtx)
	return context.WithValue(ctx, ctxNetpollCon{}, ch)
}
