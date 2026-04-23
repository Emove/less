package client

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/codec"
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
	closeCount int32
}

func (t *fakeTransport) Listen(string, transport.EventDriver) error {
	return errors.New("unexpected Listen call")
}

func (t *fakeTransport) Dial(network, addr string, driver transport.EventDriver) error {
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
