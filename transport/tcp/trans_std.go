package tcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/emove/less/internal/recovery"
	"github.com/emove/less/log"
	trans "github.com/emove/less/transport"
)

type transport struct {
	ctx      context.Context
	cancel   context.CancelFunc
	ops      *TCPOptions
	listener net.Listener
	conns    sync.Map
	closed   int32
	mu       sync.Mutex
}

var _ trans.Transport = (*transport)(nil)

func New(op ...trans.Option) trans.Transport {
	ops := cloneDefaultOptions()
	for _, o := range op {
		o(ops)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &transport{
		ctx:    ctx,
		cancel: cancel,
		ops:    ops,
	}
}

func (t *transport) Listen(addr string, driver trans.EventDriver) error {
	if t.isClosed() {
		return net.ErrClosed
	}

	tcpAddr, err := net.ResolveTCPAddr(t.ops.Network, addr)
	if err != nil {
		return err
	}
	listener, err := net.ListenTCP(tcpAddr.Network(), tcpAddr)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		t.clearListener(listener)
	}()

	t.mu.Lock()
	if t.isClosed() {
		t.mu.Unlock()
		_ = listener.Close()
		return nil
	}
	t.listener = listener
	t.mu.Unlock()

	log.Infof(fmt.Sprintf("transport listening, network: %s, address: %s", t.ops.Network, addr))

	var con net.Conn
	for {
		con, err = listener.Accept()
		if err != nil {
			if t.isClosed() || errors.Is(err, net.ErrClosed) {
				return nil
			}
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
		t.conns.Store(wrapped, struct{}{})
		cc, err = driver.OnConnect(cc, wrapped)
		if err != nil {
			t.conns.Delete(wrapped)
			_ = wrapped.Close()
			continue
		}

		go t.readLoop(cc, wrapped, driver)
	}
}

func (t *transport) Dial(network, addr string, driver trans.EventDriver) error {
	if t.isClosed() {
		return net.ErrClosed
	}

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
	t.conns.Store(wrapped, struct{}{})
	if cc, err = driver.OnConnect(cc, wrapped); err != nil {
		t.conns.Delete(wrapped)
		_ = wrapped.Close()
		return err
	}

	go t.readLoop(cc, wrapped, driver)
	return nil
}

func (t *transport) Close() {
	if !atomic.CompareAndSwapInt32(&t.closed, 0, 1) {
		return
	}
	t.cancel()

	t.mu.Lock()
	listener := t.listener
	t.listener = nil
	t.mu.Unlock()
	if listener != nil {
		_ = listener.Close()
	}

	t.conns.Range(func(key, _ interface{}) bool {
		conn := key.(trans.Connection)
		t.conns.Delete(conn)
		_ = conn.Close()
		return true
	})
}

func (t *transport) readLoop(ctx context.Context, conn trans.Connection, driver trans.EventDriver) {
	var closeErr error
	defer func() {
		recovery.Recover(func(err error) {
			closeErr = err
		})
		t.conns.Delete(conn)
		_ = conn.Close()
		driver.OnConnClosed(ctx, conn, closeErr)
	}()

	for {
		if t.isClosed() || !conn.IsActive() {
			return
		}

		if err := driver.OnMessage(ctx, conn); err != nil {
			closeErr = err
			return
		}

		select {
		case <-t.ctx.Done():
			return
		default:
		}
	}
}

func (t *transport) clearListener(listener net.Listener) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.listener == listener {
		t.listener = nil
	}
}

func (t *transport) isClosed() bool {
	return atomic.LoadInt32(&t.closed) == 1
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
