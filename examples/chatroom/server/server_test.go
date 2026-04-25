package main

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/examples/chatroom/chat"
)

const defaultWaitTimeout = 3 * time.Second

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
	addr := reserveTCPAddr(t)
	srv := newChatServer(addr, newHub())
	srv.Run()
	t.Cleanup(func() { srv.Shutdown(context.Background(), nil) })

	inbound := make(chan *chat.Message, 1)
	cli := client.NewClient(
		"tcp",
		addr,
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(chat.NewJSONCodec()),
		client.WithRouter(func(context.Context, less.Channel, interface{}) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, msg interface{}) error {
				message, ok := msg.(*chat.Message)
				if !ok {
					return nil
				}
				select {
				case inbound <- message:
				default:
				}
				return nil
			}, nil
		}),
	)
	t.Cleanup(func() { cli.Close(nil) })

	if err := dialChatClientEventually(t, cli, context.Background()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	if cli.Channel() == nil {
		t.Fatal("client Channel() = nil, want active channel")
	}
	if err := cli.Channel().Write(chat.SetName("alice")); err != nil {
		t.Fatalf("Write SetName failed: %v", err)
	}

	want := chat.System("alice joined")
	select {
	case got := <-inbound:
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("inbound message = %#v, want %#v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for inbound message %#v", want)
	}
}

func TestCheckListenAddressAvailableReturnsErrorForOccupiedAddress(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	if err := checkListenAddressAvailable(listener.Addr().String()); err == nil {
		t.Fatal("checkListenAddressAvailable() error = nil, want error")
	}
}

func TestWaitForServerReadyReturnsWhenAddressDialable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
		accepted <- struct{}{}
	}()

	if err := waitForServerReady(listener.Addr().String(), time.Second); err != nil {
		t.Fatalf("waitForServerReady() error = %v, want nil", err)
	}
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("waitForServerReady() did not dial listener")
	}
}

func TestChatroomTwoClientsBroadcastAndLeave(t *testing.T) {
	addr := reserveTCPAddr(t)
	h := newHub()
	srv := newChatServer(addr, h)
	srv.Run()
	t.Cleanup(func() { srv.Shutdown(context.Background(), nil) })

	aliceMessages := make(chan *chat.Message, 8)
	bobMessages := make(chan *chat.Message, 8)
	alice := newTestClient(t, addr, aliceMessages)
	bob := newTestClient(t, addr, bobMessages)
	t.Cleanup(func() {
		alice.Close(nil)
		bob.Close(nil)
	})

	if err := dialChatClientEventually(t, alice, context.Background()); err != nil {
		t.Fatalf("alice dial: %v", err)
	}
	if err := dialChatClientEventually(t, bob, context.Background()); err != nil {
		t.Fatalf("bob dial: %v", err)
	}

	aliceChannel := alice.Channel()
	bobChannel := bob.Channel()
	if aliceChannel == nil || bobChannel == nil {
		t.Fatal("expected both clients to have active channels")
	}

	if err := aliceChannel.Write(chat.SetName("alice")); err != nil {
		t.Fatalf("alice set name: %v", err)
	}
	if err := bobChannel.Write(chat.SetName("bob")); err != nil {
		t.Fatalf("bob set name: %v", err)
	}

	waitUntil(t, func() bool {
		return len(aliceMessages) >= 2 && len(bobMessages) >= 2
	}, "expected both clients to receive join messages")

	if err := aliceChannel.Write(chat.Chat("", "hello")); err != nil {
		t.Fatalf("alice chat: %v", err)
	}

	gotChat := receiveMatchingMessage(t, bobMessages, "bob chat broadcast", func(msg *chat.Message) bool {
		return reflect.DeepEqual(msg, chat.Chat("alice", "hello"))
	})
	if !reflect.DeepEqual(gotChat, chat.Chat("alice", "hello")) {
		t.Fatalf("bob chat = %#v, want %#v", gotChat, chat.Chat("alice", "hello"))
	}

	alice.Close(nil)

	gotLeave := receiveMatchingMessage(t, bobMessages, "bob leave message", func(msg *chat.Message) bool {
		return msg.Type == chat.TypeSystem && msg.Text == "alice left"
	})
	if !reflect.DeepEqual(gotLeave, chat.System("alice left")) {
		t.Fatalf("bob leave = %#v, want %#v", gotLeave, chat.System("alice left"))
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	defer func() { _ = listener.Close() }()

	return listener.Addr().String()
}

func waitUntil(t *testing.T, predicate func() bool, failure string) {
	t.Helper()

	deadline := time.Now().Add(defaultWaitTimeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !predicate() {
		t.Fatal(failure)
	}
}

func receiveMatchingMessage(t *testing.T, messages <-chan *chat.Message, description string, match func(*chat.Message) bool) *chat.Message {
	t.Helper()

	deadline := time.After(defaultWaitTimeout)
	for {
		select {
		case msg := <-messages:
			if match(msg) {
				return msg
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", description)
			return nil
		}
	}
}

func newTestClient(t *testing.T, addr string, inbound chan<- *chat.Message) *client.Client {
	t.Helper()

	return client.NewClient(
		"tcp",
		addr,
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(chat.NewJSONCodec()),
		client.WithRouter(func(context.Context, less.Channel, interface{}) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, msg interface{}) error {
				message, ok := msg.(*chat.Message)
				if !ok {
					return fmt.Errorf("unexpected message type %T", msg)
				}
				inbound <- message
				return nil
			}, nil
		}),
	)
}

func dialChatClientEventually(t *testing.T, cli *client.Client, ctx context.Context) error {
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
