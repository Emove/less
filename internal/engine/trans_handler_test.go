package engine

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/internal/engine/framebuf"
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
	readChunks  [][]byte
	writeErr    error
}

func (c *handlerTestConn) Read(buf []byte) (int, error) {
	if len(c.readChunks) == 0 {
		return 0, io.EOF
	}
	chunk := c.readChunks[0]
	c.readChunks = c.readChunks[1:]
	n := copy(buf, chunk)
	return n, nil
}
func (c *handlerTestConn) Write(buf []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return len(buf), nil
}
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

type trackingFrame struct {
	bytes        []byte
	releaseCount *int32
}

func (f *trackingFrame) Bytes() []byte { return f.bytes }
func (f *trackingFrame) Retain()       {}
func (f *trackingFrame) Release() {
	atomic.AddInt32(f.releaseCount, 1)
}

type stubPacketCodec struct {
	frame codec.Frame
	err   error
}

func (s stubPacketCodec) Name() string { return "stub-packet" }
func (s stubPacketCodec) Encode(dst codec.WriterBuffer, payload codec.Frame) error {
	return nil
}
func (s stubPacketCodec) Decode(src codec.ReaderBuffer) (codec.Frame, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.frame, nil
}

type stubPayloadCodec struct {
	unmarshalErr   error
	unmarshalPanic error
}

func (s stubPayloadCodec) Name() string { return "stub-payload" }
func (s stubPayloadCodec) Marshal(message any) (codec.Frame, error) {
	return framebuf.NewFrame([]byte("ignored")), nil
}
func (s stubPayloadCodec) Unmarshal(payload codec.Frame) (any, error) {
	if s.unmarshalPanic != nil {
		panic(s.unmarshalPanic)
	}
	if s.unmarshalErr != nil {
		return nil, s.unmarshalErr
	}
	return string(payload.Bytes()), nil
}

func TestSrvTransHandler_OnMessage_ReleasesFrameWhenUnmarshalFails(t *testing.T) {
	var releaseCount int32

	handler := NewSrvTransHandler(
		context.Background(),
		WithRouter(testRouter()),
		WithPacketCodec(stubPacketCodec{
			frame: &trackingFrame{
				bytes:        []byte("bad"),
				releaseCount: &releaseCount,
			},
		}),
		WithPayloadCodec(stubPayloadCodec{unmarshalErr: errors.New("boom")}),
	)

	conn := &handlerTestConn{}
	ctx, err := handler.OnConnect(context.Background(), conn)
	if err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	err = handler.OnMessage(ctx, conn)
	if err == nil {
		t.Fatal("expected OnMessage to return unmarshal error")
	}

	if got := atomic.LoadInt32(&releaseCount); got != 1 {
		t.Fatalf("frame Release() count = %d, want 1", got)
	}

	if conn.IsActive() {
		t.Fatal("expected unmarshal error to close the connection")
	}
}

func TestSrvTransHandler_OnMessage_ReleasesFrameWhenUnmarshalPanics(t *testing.T) {
	var releaseCount int32

	handler := NewSrvTransHandler(
		context.Background(),
		WithRouter(testRouter()),
		WithPacketCodec(stubPacketCodec{
			frame: &trackingFrame{
				bytes:        []byte("bad"),
				releaseCount: &releaseCount,
			},
		}),
		WithPayloadCodec(stubPayloadCodec{unmarshalPanic: errors.New("panic")}),
	)

	conn := &handlerTestConn{}
	ctx, err := handler.OnConnect(context.Background(), conn)
	if err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	_ = handler.OnMessage(ctx, conn)

	if got := atomic.LoadInt32(&releaseCount); got != 1 {
		t.Fatalf("frame Release() count = %d, want 1", got)
	}

	if conn.IsActive() {
		t.Fatal("expected panic path to close the connection")
	}
}

type sequentialPacketCodec struct{}

func (sequentialPacketCodec) Name() string { return "sequential-packet" }
func (sequentialPacketCodec) Encode(dst codec.WriterBuffer, payload codec.Frame) error {
	return nil
}
func (sequentialPacketCodec) Decode(src codec.ReaderBuffer) (codec.Frame, error) {
	header, err := src.Next(1)
	if err != nil {
		return nil, err
	}
	return src.Slice(int(header[0]))
}

func TestSrvTransHandler_OnMessage_ReusesReaderAcrossCalls(t *testing.T) {
	var handled []string
	router := func(ctx context.Context, ch less.Channel, msg interface{}) (less.Handler, error) {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			handled = append(handled, message.(string))
			return nil
		}, nil
	}

	conn := &handlerTestConn{
		readChunks: [][]byte{
			[]byte{5, 'h', 'e', 'l', 'l', 'o', 5, 'w', 'o', 'r', 'l', 'd'},
		},
	}
	handler := NewSrvTransHandler(
		context.Background(),
		WithRouter(router),
		WithPacketCodec(sequentialPacketCodec{}),
		WithPayloadCodec(payload.NewTextCodec()),
	)

	ctx, err := handler.OnConnect(context.Background(), conn)
	if err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	if err := handler.OnMessage(ctx, conn); err != nil {
		t.Fatalf("first OnMessage failed: %v", err)
	}
	if err := handler.OnMessage(ctx, conn); err != nil {
		t.Fatalf("second OnMessage failed: %v", err)
	}

	want := []string{"hello", "world"}
	if len(handled) != len(want) {
		t.Fatalf("handled %v, want %v", handled, want)
	}
	for i := range want {
		if handled[i] != want[i] {
			t.Fatalf("handled[%d] = %q, want %q", i, handled[i], want[i])
		}
	}
}

func TestWriteHandler_ClosesChannelWhenFlushFails(t *testing.T) {
	var closedCount int32
	conn := &handlerTestConn{writeErr: errors.New("flush failed")}
	handler := NewSrvTransHandler(
		context.Background(),
		WithRouter(testRouter()),
		AddOnChannelClosed(func(ctx context.Context, ch less.Channel, err error) {
			atomic.AddInt32(&closedCount, 1)
		}),
	)

	ctx, err := handler.OnConnect(context.Background(), conn)
	if err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	ch := ctx.Value(ctxChannelKey{}).(less.Channel)
	err = ch.Write("hello")
	if err == nil || !strings.Contains(err.Error(), "flush failed") {
		t.Fatalf("Write() error = %v, want flush failure", err)
	}

	if got := atomic.LoadInt32(&closedCount); got != 1 {
		t.Fatalf("OnChannelClosed called %d times, want 1", got)
	}
	if conn.IsActive() {
		t.Fatal("expected flush failure to close the connection")
	}
}
