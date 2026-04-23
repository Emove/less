package client

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	engine "github.com/emove/less/internal/engine"
	"github.com/emove/less/server"
	"github.com/emove/less/transport"
)

var _ interface {
	Dial(context.Context) error
	Channel() less.Channel
	Close(error)
} = (*Client)(nil)

func TestClient_DialEstablishesChannel(t *testing.T) {
	addr := reserveTCPAddr(t)

	serverOnChannelCalled := make(chan struct{}, 1)
	srv := server.NewServer(
		addr,
		server.WithRouter(noopRouter()),
		server.WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			select {
			case serverOnChannelCalled <- struct{}{}:
			default:
			}
			return ctx, nil
		}),
	)
	srv.Run()
	t.Cleanup(func() { srv.Shutdown(context.Background(), nil) })

	var clientOnChannelCalled int32
	cli := NewClient(
		"tcp",
		addr,
		WithRouter(noopRouter()),
		WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			atomic.StoreInt32(&clientOnChannelCalled, 1)
			return ctx, nil
		}),
	)
	t.Cleanup(func() { cli.Close(nil) })

	if err := dialClientEventually(t, cli, context.Background()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	select {
	case <-serverOnChannelCalled:
	case <-time.After(time.Second):
		t.Fatal("expected server OnChannel hook to run")
	}
	if atomic.LoadInt32(&clientOnChannelCalled) != 1 {
		t.Fatal("expected client OnChannel hook to run")
	}
	if cli.Channel() == nil {
		t.Fatal("expected active channel to be captured")
	}
}

func TestClient_DialReturnsOnChannelError(t *testing.T) {
	sentinel := errors.New("on channel failed")
	trans := &fakeTransport{}
	conn := &fakeConn{}

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		_, err := driver.OnConnect(context.Background(), conn)
		return err
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
		WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			return ctx, sentinel
		}),
	)
	t.Cleanup(func() { cli.Close(nil) })

	err := cli.Dial(context.Background())
	if !errors.Is(err, sentinel) {
		t.Fatalf("Dial error = %v, want %v", err, sentinel)
	}
	if cli.Channel() != nil {
		t.Fatal("expected failed dial to leave no active channel")
	}
}

func TestClient_DialRejectsRedialWhileSessionActive(t *testing.T) {
	trans := &fakeTransport{}
	conn := &fakeConn{}

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		_, err := driver.OnConnect(context.Background(), conn)
		return err
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
	)
	t.Cleanup(func() { cli.Close(nil) })

	if err := cli.Dial(context.Background()); err != nil {
		t.Fatalf("first Dial failed: %v", err)
	}

	firstChannel := cli.Channel()
	if firstChannel == nil {
		t.Fatal("expected first dial to capture an active channel")
	}

	err := cli.Dial(context.Background())
	if err == nil {
		t.Fatal("expected redial to be rejected while session is active")
	}
	if got := cli.Channel(); got != firstChannel {
		t.Fatal("expected redial rejection to preserve the active channel")
	}
	if got := atomic.LoadInt32(&trans.dialCalls); got != 1 {
		t.Fatalf("transport Dial called %d times, want 1", got)
	}
}

func TestClient_DialContextCancellationClosesCurrentSession(t *testing.T) {
	trans := &fakeTransport{}
	conn := &fakeConn{}
	done := make(chan error, 1)

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		_, err := driver.OnConnect(context.Background(), conn)
		return err
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
		WithOnChannelClosed(func(_ context.Context, _ less.Channel, err error) {
			select {
			case done <- err:
			default:
			}
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { cli.Close(nil) })

	if err := cli.Dial(ctx); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	if cli.Channel() == nil {
		t.Fatal("expected active channel after successful dial")
	}

	cancel()

	waitFor(t, time.Second, func() bool {
		return cli.Channel() == nil && !conn.IsActive()
	})

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("OnChannelClosed error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("expected OnChannelClosed to observe context cancellation")
	}
}

func TestClient_RemoteDisconnectClearsSessionAndAllowsRedial(t *testing.T) {
	trans := &fakeTransport{}
	var connectCtx context.Context
	firstConn := &fakeConn{}
	secondConn := &fakeConn{}

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		var conn transport.Connection
		if atomic.LoadInt32(&trans.dialCalls) == 1 {
			conn = firstConn
		} else {
			conn = secondConn
		}

		var err error
		connectCtx, err = driver.OnConnect(context.Background(), conn)
		return err
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
	)
	t.Cleanup(func() { cli.Close(nil) })

	if err := cli.Dial(context.Background()); err != nil {
		t.Fatalf("first Dial failed: %v", err)
	}

	firstChannel := cli.Channel()
	if firstChannel == nil {
		t.Fatal("expected first dial to capture an active channel")
	}

	trans.driver.OnConnClosed(connectCtx, firstConn, io.EOF)

	waitFor(t, time.Second, func() bool {
		return cli.Channel() == nil
	})

	if err := cli.Dial(context.Background()); err != nil {
		t.Fatalf("second Dial failed after remote disconnect: %v", err)
	}

	if got := atomic.LoadInt32(&trans.dialCalls); got != 2 {
		t.Fatalf("transport Dial called %d times, want 2", got)
	}
	if cli.Channel() == nil {
		t.Fatal("expected redial to capture a new active channel")
	}
	if cli.Channel() == firstChannel {
		t.Fatal("expected redial to replace the retired channel")
	}
}

func TestClient_CloseDuringDialPreventsOrphanedSession(t *testing.T) {
	originalNewEndpointHandler := newEndpointHandler
	t.Cleanup(func() {
		newEndpointHandler = originalNewEndpointHandler
	})

	handlerBuilt := make(chan struct{})
	releaseHandler := make(chan struct{})
	newEndpointHandler = func(ctx context.Context, ops ...engine.Option) engine.TransHandler {
		handler := originalNewEndpointHandler(ctx, ops...)
		close(handlerBuilt)
		<-releaseHandler
		return handler
	}

	trans := &fakeTransport{}
	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		_, err := driver.OnConnect(context.Background(), &fakeConn{})
		return err
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
	)
	t.Cleanup(func() { cli.Close(nil) })

	dialDone := make(chan error, 1)
	go func() {
		dialDone <- cli.Dial(context.Background())
	}()

	<-handlerBuilt
	cli.Close(errors.New("client closed during dial"))
	close(releaseHandler)

	select {
	case err := <-dialDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Dial error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("Dial did not return after Close")
	}

	if cli.Channel() != nil {
		t.Fatal("expected retired dial to leave no active channel")
	}
	if got := atomic.LoadInt32(&trans.dialCalls); got != 0 {
		t.Fatalf("transport Dial called %d times, want 0", got)
	}
}

func TestClient_CloseAfterLateSuccessfulDialDoesNotLeaveSessionActive(t *testing.T) {
	trans := &fakeTransport{}
	conn := &fakeConn{}
	connected := make(chan struct{})
	releaseDialReturn := make(chan struct{})
	var closedCount int32

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		if _, err := driver.OnConnect(context.Background(), conn); err != nil {
			return err
		}
		close(connected)
		<-releaseDialReturn
		return nil
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
		WithOnChannelClosed(func(context.Context, less.Channel, error) {
			atomic.AddInt32(&closedCount, 1)
		}),
	)
	t.Cleanup(func() { cli.Close(nil) })

	dialDone := make(chan error, 1)
	go func() {
		dialDone <- cli.Dial(context.Background())
	}()

	select {
	case <-connected:
	case <-time.After(time.Second):
		t.Fatal("Dial did not reach connected state")
	}

	if got := atomic.LoadInt32(&trans.dialCalls); got != 1 {
		t.Fatalf("transport Dial called %d times, want 1", got)
	}
	if cli.Channel() == nil {
		t.Fatal("expected late-success dial to publish a channel before Close")
	}

	cli.Close(errors.New("close during late dial"))
	close(releaseDialReturn)

	select {
	case err := <-dialDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Dial error = %v, want %v", err, context.Canceled)
		}
	case <-time.After(time.Second):
		t.Fatal("Dial did not return after Close")
	}

	if cli.Channel() != nil {
		t.Fatal("expected retired late-success dial to leave no active channel")
	}
	if conn.IsActive() {
		t.Fatal("expected Close to close the late-connected underlying connection")
	}
	if got := atomic.LoadInt32(&closedCount); got != 1 {
		t.Fatalf("OnChannelClosed called %d times, want 1", got)
	}
}

func TestClient_StaleSessionCloseCallbackDoesNotClearNewSession(t *testing.T) {
	trans := &fakeTransport{}
	firstConn := &fakeConn{}
	secondConn := &fakeConn{}
	var firstCtx context.Context
	var secondCtx context.Context

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		var conn transport.Connection
		if atomic.LoadInt32(&trans.dialCalls) == 1 {
			conn = firstConn
		} else {
			conn = secondConn
		}

		connectCtx, err := driver.OnConnect(context.Background(), conn)
		if err != nil {
			return err
		}
		if atomic.LoadInt32(&trans.dialCalls) == 1 {
			firstCtx = connectCtx
		} else {
			secondCtx = connectCtx
		}
		return nil
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
	)
	t.Cleanup(func() { cli.Close(nil) })

	if err := cli.Dial(context.Background()); err != nil {
		t.Fatalf("first Dial failed: %v", err)
	}
	firstDriver := trans.driver

	trans.driver.OnConnClosed(firstCtx, firstConn, io.EOF)
	waitFor(t, time.Second, func() bool {
		return cli.Channel() == nil
	})

	if err := cli.Dial(context.Background()); err != nil {
		t.Fatalf("second Dial failed: %v", err)
	}

	secondChannel := cli.Channel()
	if secondChannel == nil {
		t.Fatal("expected second dial to capture an active channel")
	}

	firstDriver.OnConnClosed(firstCtx, firstConn, io.EOF)

	if got := cli.Channel(); got != secondChannel {
		t.Fatal("expected stale old-session close callback to preserve the newer active channel")
	}
	if !secondConn.IsActive() {
		t.Fatal("expected stale old-session close callback to leave the newer connection active")
	}
	if secondCtx == nil {
		t.Fatal("expected second session context to be captured")
	}
}

func TestClient_CodecOptionsRejectTypedNil(t *testing.T) {
	t.Run("packet", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected WithPacketCodec to panic on typed nil")
			}
		}()
		var c *typedNilPacketCodec
		_ = WithPacketCodec(c)
	})

	t.Run("payload", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected WithPayloadCodec to panic on typed nil")
			}
		}()
		var c *typedNilPayloadCodec
		_ = WithPayloadCodec(c)
	})
}

func TestClient_WithTransportRejectsNil(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected WithTransport to panic on nil")
			}
		}()
		_ = WithTransport(nil)
	})

	t.Run("typed nil", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected WithTransport to panic on typed nil")
			}
		}()
		var tr *typedNilTransport
		_ = WithTransport(tr)
	})
}

func TestClient_ClosePropagatesCallerErrorOnce(t *testing.T) {
	trans := &fakeTransport{}
	conn := &fakeConn{}
	customErr := errors.New("client shutdown")
	var closedCount int32
	var closedErr error

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		_, err := driver.OnConnect(context.Background(), conn)
		return err
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
		WithOnChannelClosed(func(context.Context, less.Channel, error) {
			atomic.AddInt32(&closedCount, 1)
		}),
		WithOnChannelClosed(func(_ context.Context, _ less.Channel, err error) {
			closedErr = err
		}),
	)

	if err := cli.Dial(context.Background()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	cli.Close(customErr)

	if got := atomic.LoadInt32(&closedCount); got != 1 {
		t.Fatalf("OnChannelClosed called %d times, want 1", got)
	}
	if !errors.Is(closedErr, customErr) {
		t.Fatalf("OnChannelClosed error = %v, want %v", closedErr, customErr)
	}
	if cli.Channel() != nil {
		t.Fatal("expected active channel to be cleared after Close")
	}
	if conn.IsActive() {
		t.Fatal("expected Close to close the underlying connection")
	}
}

func TestClient_CloseIsIdempotent(t *testing.T) {
	trans := &fakeTransport{}
	conn := &fakeConn{}
	customErr := errors.New("client shutdown")
	var closedCount int32
	var closedErr error

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		_, err := driver.OnConnect(context.Background(), conn)
		return err
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
		WithOnChannelClosed(func(context.Context, less.Channel, error) {
			atomic.AddInt32(&closedCount, 1)
		}),
		WithOnChannelClosed(func(_ context.Context, _ less.Channel, err error) {
			closedErr = err
		}),
	)

	if err := cli.Dial(context.Background()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	cli.Close(customErr)
	cli.Close(customErr)

	if got := atomic.LoadInt32(&closedCount); got != 1 {
		t.Fatalf("OnChannelClosed called %d times, want 1", got)
	}
	if !errors.Is(closedErr, customErr) {
		t.Fatalf("OnChannelClosed error = %v, want %v", closedErr, customErr)
	}
	if cli.Channel() != nil {
		t.Fatal("expected active channel to remain cleared after repeated Close")
	}
	if conn.IsActive() {
		t.Fatal("expected repeated Close to leave the underlying connection closed")
	}
}

func TestClient_OptionsSmoke(t *testing.T) {
	_ = NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithRouter(noopRouter()),
		WithInboundMiddleware(noopMiddleware()),
		WithOutboundMiddleware(noopMiddleware()),
		WithOnChannelClosed(func(context.Context, less.Channel, error) {}),
		MaxSendMessageSize(128),
		MaxReceiveMessageSize(256),
	)
}

func noopRouter() less.Router {
	return func(context.Context, less.Channel, interface{}) (less.Handler, error) {
		return func(context.Context, less.Channel, interface{}) error {
			return nil
		}, nil
	}
}

func noopMiddleware() less.Middleware {
	return func(next less.Handler) less.Handler {
		return func(ctx context.Context, ch less.Channel, msg interface{}) error {
			return next(ctx, ch, msg)
		}
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	return listener.Addr().String()
}

func dialClientEventually(t *testing.T, cli *Client, ctx context.Context) error {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var lastErr error
	for {
		err := cli.Dial(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type fakeTransport struct {
	driver     transport.EventDriver
	dial       func(network, addr string, driver transport.EventDriver) error
	dialCalls  int32
	closeCount int32
}

func (t *fakeTransport) Listen(string, transport.EventDriver) error {
	return errors.New("unexpected Listen call")
}

func (t *fakeTransport) Dial(network, addr string, driver transport.EventDriver) error {
	atomic.AddInt32(&t.dialCalls, 1)
	t.driver = driver
	if t.dial != nil {
		return t.dial(network, addr, driver)
	}
	return nil
}

func (t *fakeTransport) Close() {
	atomic.AddInt32(&t.closeCount, 1)
}

type fakeAddr struct{}

func (fakeAddr) Network() string { return "tcp" }
func (fakeAddr) String() string  { return "127.0.0.1:18888" }

type fakeConn struct {
	closed int32
}

func (c *fakeConn) Read(buf []byte) (int, error)  { return 0, nil }
func (c *fakeConn) Write(buf []byte) (int, error) { return len(buf), nil }
func (c *fakeConn) IsActive() bool                { return atomic.LoadInt32(&c.closed) == 0 }
func (c *fakeConn) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	return nil
}
func (c *fakeConn) LocalAddr() net.Addr  { return fakeAddr{} }
func (c *fakeConn) RemoteAddr() net.Addr { return fakeAddr{} }

type typedNilPacketCodec struct{}

func (*typedNilPacketCodec) Name() string { return "typed-nil-packet" }
func (*typedNilPacketCodec) Encode(codec.WriterBuffer, codec.Frame) error {
	return nil
}
func (*typedNilPacketCodec) Decode(codec.ReaderBuffer) (codec.Frame, error) {
	return nil, nil
}

type typedNilPayloadCodec struct{}

func (*typedNilPayloadCodec) Name() string { return "typed-nil-payload" }
func (*typedNilPayloadCodec) Marshal(any) (codec.Frame, error) {
	return nil, nil
}
func (*typedNilPayloadCodec) Unmarshal(codec.Frame) (any, error) {
	return nil, nil
}

type typedNilTransport struct{}

func (*typedNilTransport) Listen(string, transport.EventDriver) error { return nil }
func (*typedNilTransport) Dial(string, string, transport.EventDriver) error {
	return nil
}
func (*typedNilTransport) Close() {}

func waitFor(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !predicate() {
		t.Fatal("condition not met before timeout")
	}
}
