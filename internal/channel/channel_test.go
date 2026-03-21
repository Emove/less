package channel

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"

	"github.com/emove/less"
	"github.com/emove/less/io"
)

// mockAddr implements net.Addr for testing.
type mockAddr struct{}

func (mockAddr) Network() string { return "tcp" }
func (mockAddr) String() string  { return "127.0.0.1:9999" }

// mockConn implements transport.Connection for testing.
type mockConn struct {
	active int32
}

func (m *mockConn) Read(buf []byte) (int, error)  { return 0, nil }
func (m *mockConn) Reader() io.Reader              { return nil }
func (m *mockConn) Writer() io.Writer              { return nil }
func (m *mockConn) IsActive() bool                 { return atomic.LoadInt32(&m.active) == 0 }
func (m *mockConn) Close() error                   { atomic.StoreInt32(&m.active, 1); return nil }
func (m *mockConn) LocalAddr() net.Addr            { return mockAddr{} }
func (m *mockConn) RemoteAddr() net.Addr           { return mockAddr{} }

// ============================== Test 7.1: Close() idempotency ============================== //

func TestChannel_Close_Idempotent(t *testing.T) {
	var closedCount int32

	onChannelClosed := func(ctx context.Context, ch less.Channel, err error) {
		atomic.AddInt32(&closedCount, 1)
	}

	factory := NewPipelineFactory(nil, []less.OnChannelClosed{onChannelClosed}, nil, nil, nil, nil)
	ch := NewChannel(&mockConn{}, factory)
	if err := ch.Activate(context.Background()); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	// Call Close three times — only the first should trigger the callback
	ch.Close(errors.New("first close"))
	ch.Close(errors.New("second close"))
	ch.Close(errors.New("third close"))

	if count := atomic.LoadInt32(&closedCount); count != 1 {
		t.Errorf("OnChannelClosed called %d times, want 1", count)
	}
}

// ============================== Test 7.2: Pipeline middleware execution order ============================== //

func makeMW(name string, order *[]string) less.Middleware {
	return func(next less.Handler) less.Handler {
		return func(ctx context.Context, ch less.Channel, msg interface{}) error {
			*order = append(*order, name)
			return next(ctx, ch, msg)
		}
	}
}

func TestPipeline_InboundOrder(t *testing.T) {
	var order []string

	globalIn := []less.Middleware{makeMW("globalIn", &order)}
	routerMW := makeMW("router", &order)

	factory := NewPipelineFactory(nil, nil, globalIn, nil, routerMW, nil)
	ch := NewChannel(&mockConn{}, factory)
	if err := ch.Activate(context.Background()); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	// Add channel-specific inbound middleware
	ch.AddInboundMiddleware(makeMW("chIn", &order))

	if err := ch.TriggerInbound("test msg"); err != nil {
		t.Fatalf("TriggerInbound failed: %v", err)
	}

	expected := []string{"globalIn", "chIn", "router"}
	if len(order) != len(expected) {
		t.Fatalf("got %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %s, want %s", i, order[i], v)
		}
	}
}

func TestPipeline_OutboundOrder(t *testing.T) {
	var order []string

	globalOut := []less.Middleware{makeMW("globalOut", &order)}
	writeHandler := func(ctx context.Context, ch less.Channel, msg interface{}) error {
		order = append(order, "writeHandler")
		return nil
	}

	factory := NewPipelineFactory(nil, nil, nil, globalOut, nil, writeHandler)
	ch := NewChannel(&mockConn{}, factory)
	if err := ch.Activate(context.Background()); err != nil {
		t.Fatalf("Activate failed: %v", err)
	}

	// Add channel-specific outbound middleware
	ch.AddOutboundMiddleware(makeMW("chOut", &order))

	if err := ch.Write("test msg"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	expected := []string{"chOut", "globalOut", "writeHandler"}
	if len(order) != len(expected) {
		t.Fatalf("got %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %s, want %s", i, order[i], v)
		}
	}
}
