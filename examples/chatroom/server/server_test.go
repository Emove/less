package main

import (
	"context"
	"net"
	"reflect"
	"sync"
	"testing"

	"github.com/emove/less"
	"github.com/emove/less/examples/chatroom/chat"
)

type mockAddr struct{}

func (mockAddr) Network() string { return "tcp" }
func (mockAddr) String() string  { return "127.0.0.1:9999" }

type mockChannel struct {
	mu       sync.Mutex
	writes   []*chat.Message
	closed   bool
	closeErr error
}

func (m *mockChannel) Context() context.Context                   { return context.Background() }
func (m *mockChannel) RemoteAddr() net.Addr                       { return mockAddr{} }
func (m *mockChannel) LocalAddr() net.Addr                        { return mockAddr{} }
func (m *mockChannel) IsActive() bool                             { return !m.closed }
func (m *mockChannel) AddOnChannelClosed(...less.OnChannelClosed) {}
func (m *mockChannel) AddInboundMiddleware(...less.Middleware)    {}
func (m *mockChannel) AddOutboundMiddleware(...less.Middleware)   {}
func (m *mockChannel) Close(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.closeErr = err
}
func (m *mockChannel) Write(msg interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	typed, ok := msg.(*chat.Message)
	if !ok {
		return nil
	}
	m.writes = append(m.writes, typed)
	return nil
}
func (m *mockChannel) snapshot() []*chat.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*chat.Message, len(m.writes))
	copy(out, m.writes)
	return out
}

func TestHubRegisterSetNameBroadcastAndUnregister(t *testing.T) {
	h := newHub()
	alice := &mockChannel{}
	bob := &mockChannel{}

	h.register(alice)
	h.register(bob)

	if joined := h.setName(alice, "alice"); joined != "alice" {
		t.Fatalf("setName() = %q, want alice", joined)
	}
	h.broadcast(chat.Chat("alice", "hello"))

	wantBroadcast := []*chat.Message{
		chat.Chat("alice", "hello"),
	}
	if got := bob.snapshot(); !reflect.DeepEqual(got, wantBroadcast) {
		t.Fatalf("bob messages = %#v, want %#v", got, wantBroadcast)
	}

	left, ok := h.unregister(alice)
	if !ok {
		t.Fatal("unregister() ok = false, want true")
	}
	if left != "alice" {
		t.Fatalf("unregister() name = %q, want alice", left)
	}
}

func TestChatHandlerRejectsUnnamedChannel(t *testing.T) {
	h := newHub()
	ch := &mockChannel{}
	h.register(ch)

	err := chatHandler(h)(context.Background(), ch, chat.Chat("", "hello"))
	if err != nil {
		t.Fatalf("chatHandler() error = %v", err)
	}

	want := []*chat.Message{chat.System("set a name before chatting")}
	if got := ch.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("messages = %#v, want %#v", got, want)
	}
}

func TestRouterSelectsHandlersByType(t *testing.T) {
	h := newHub()
	router := newRouter(h)

	t.Run("set name", func(t *testing.T) {
		ch := &mockChannel{}
		h.register(ch)

		handler, err := router(context.Background(), ch, chat.SetName("alice"))
		if err != nil {
			t.Fatalf("router() error = %v", err)
		}

		if err := handler(context.Background(), ch, chat.SetName("alice")); err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		want := []*chat.Message{chat.System("alice joined")}
		if got := ch.snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("messages = %#v, want %#v", got, want)
		}
	})

	t.Run("chat", func(t *testing.T) {
		ch := &mockChannel{}
		h.register(ch)
		h.setName(ch, "alice")

		handler, err := router(context.Background(), ch, chat.Chat("", "hello"))
		if err != nil {
			t.Fatalf("router() error = %v", err)
		}

		if err := handler(context.Background(), ch, chat.Chat("", "hello")); err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		want := []*chat.Message{chat.Chat("alice", "hello")}
		if got := ch.snapshot(); !reflect.DeepEqual(got, want) {
			t.Fatalf("messages = %#v, want %#v", got, want)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		ch := &mockChannel{}

		_, err := router(context.Background(), ch, &chat.Message{Type: "unknown"})
		if err == nil {
			t.Fatal("router() error = nil, want error")
		}
	})
}

func TestNewChatServer(t *testing.T) {
	srv := newChatServer("127.0.0.1:0", newHub())
	if srv == nil {
		t.Fatal("newChatServer() = nil, want server")
	}
}
