package client

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	"github.com/emove/less/transport"
)

var _ interface {
	Dial(context.Context) error
	Channel() less.Channel
	Close(error)
} = (*Client)(nil)

func TestClient_DialEstablishesChannel(t *testing.T) {
	trans := &fakeTransport{}
	conn := &fakeConn{}
	var onChannelCalled int32

	trans.dial = func(network, addr string, driver transport.EventDriver) error {
		if network != "tcp" {
			t.Fatalf("network = %q, want %q", network, "tcp")
		}
		if addr != "127.0.0.1:18888" {
			t.Fatalf("addr = %q, want %q", addr, "127.0.0.1:18888")
		}
		_, err := driver.OnConnect(context.Background(), conn)
		return err
	}

	cli := NewClient(
		"tcp",
		"127.0.0.1:18888",
		WithTransport(trans),
		WithRouter(noopRouter()),
		WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			atomic.StoreInt32(&onChannelCalled, 1)
			return ctx, nil
		}),
	)
	t.Cleanup(func() { cli.Close(nil) })

	if err := cli.Dial(context.Background()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	if atomic.LoadInt32(&onChannelCalled) != 1 {
		t.Fatal("expected OnChannel hook to run")
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
