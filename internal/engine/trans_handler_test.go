package engine

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/emove/less"
	"github.com/emove/less/io"
	"github.com/emove/less/router"
)

func Test_newRouter(t *testing.T) {
	nilHandler := func(ctx context.Context, ch less.Channel, message interface{}) error {
		return nil
	}

	mw := NewRouterMiddleware(func(ctx context.Context, channel less.Channel, msg interface{}) (less.Handler, error) {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			t.Logf("router handler, handle message: %v", message)
			return nil
		}, nil
	})

	_ = mw(nilHandler)(context.Background(), nil, "router test")
}

func TestNewSrvTransHandler_RejectsMissingRouterAtConstruction(t *testing.T) {
	// The long-term contract is construction-time rejection when router is missing.
	// With the current API shape (no error return), we accept either a panic or a
	// nil handler as the rejection signal, but we do not want silent acceptance.
	var (
		panicked bool
		handler  TransHandler
	)

	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()

		handler = NewSrvTransHandler(context.Background())
	}()

	if panicked {
		return
	}

	if handler == nil {
		return
	}

	t.Fatal("expected NewSrvTransHandler to reject a missing router at construction time, but it accepted the configuration without panic or explicit failure")
}

type handlerTestAddr struct{}

func (handlerTestAddr) Network() string { return "tcp" }
func (handlerTestAddr) String() string  { return "127.0.0.1:19999" }

type handlerTestConn struct {
	closed     int32
	closeCount int32
}

func (c *handlerTestConn) Read(buf []byte) (int, error) { return 0, nil }
func (c *handlerTestConn) Reader() io.Reader            { return nil }
func (c *handlerTestConn) Writer() io.Writer            { return nil }
func (c *handlerTestConn) IsActive() bool               { return atomic.LoadInt32(&c.closed) == 0 }
func (c *handlerTestConn) Close() error {
	atomic.AddInt32(&c.closeCount, 1)
	atomic.StoreInt32(&c.closed, 1)
	return nil
}
func (c *handlerTestConn) LocalAddr() net.Addr  { return handlerTestAddr{} }
func (c *handlerTestConn) RemoteAddr() net.Addr { return handlerTestAddr{} }

func testRouter() router.Router {
	return func(ctx context.Context, ch less.Channel, msg interface{}) (less.Handler, error) {
		return func(context.Context, less.Channel, interface{}) error {
			return nil
		}, nil
	}
}

func TestSrvTransHandler_OnConnClosed_DoesNotDoubleFireClosedCallbacks(t *testing.T) {
	var closedCount int32

	handler := NewSrvTransHandler(
		context.Background(),
		WithRouter(testRouter()),
		AddOnChannelClosed(func(ctx context.Context, ch less.Channel, err error) {
			atomic.AddInt32(&closedCount, 1)
		}),
	)

	conn := &handlerTestConn{}
	ctx, err := handler.OnConnect(context.Background(), conn)
	if err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	ch := ctx.Value(ctxChannelKey{}).(less.Channel)
	ch.Close(errors.New("local close"))
	handler.OnConnClosed(ctx, conn, errors.New("remote close"))

	if got := atomic.LoadInt32(&closedCount); got != 1 {
		t.Fatalf("OnChannelClosed called %d times, want 1", got)
	}

	if conn.IsActive() {
		t.Fatal("expected channel close path to close the underlying connection")
	}

	if got := atomic.LoadInt32(&conn.closeCount); got != 1 {
		t.Fatalf("connection.Close called %d times, want 1", got)
	}
}

func TestNewSrvTransHandler_ClonesDefaultOptions(t *testing.T) {
	first := NewSrvTransHandler(
		context.Background(),
		WithRouter(testRouter()),
		AddOnChannelClosed(func(context.Context, less.Channel, error) {}),
	).(*svrTransHandler)

	second := NewSrvTransHandler(
		context.Background(),
		WithRouter(testRouter()),
	).(*svrTransHandler)

	if first.ops == second.ops {
		t.Fatal("expected NewSrvTransHandler to clone default options per instance")
	}

	if got := len(second.ops.onChannelClosed); got != 0 {
		t.Fatalf("expected isolated onChannelClosed hooks for second handler, got %d", got)
	}
}
