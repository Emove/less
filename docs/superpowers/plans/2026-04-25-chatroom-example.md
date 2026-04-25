# Chatroom Example Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a minimal runnable terminal chatroom example that uses the existing `less/server` and `less/client` APIs with JSON messages.

**Architecture:** Add an `examples/chatroom` tree with a shared `chat` package, a server `main` package, and a client `main` package. The shared package defines the JSON message model and returns the existing typed JSON payload codec; the server owns a small hub and route handlers; the client reads stdin, writes chat messages, and prints inbound broadcasts.

**Tech Stack:** Go 1.19, standard library `bufio`/`context`/`errors`/`flag`/`fmt`/`net`/`os`/`strings`/`sync`/`sync/atomic`/`testing`/`time`, existing public packages `less`, `client`, `server`, `codec/packet`, and `codec/payload`

---

## File Structure

**Create:**

- `examples/chatroom/chat/message.go`: shared message constants, message struct, constructor helpers, and JSON payload codec factory.
- `examples/chatroom/chat/message_test.go`: focused tests for the shared message codec and helper constructors.
- `examples/chatroom/server/main.go`: server CLI entrypoint.
- `examples/chatroom/server/hub.go`: server-local session registry and broadcast behavior.
- `examples/chatroom/server/handlers.go`: server `OnChannel`, `OnChannelClosed`, router, and message handlers.
- `examples/chatroom/server/server_test.go`: example-level two-client integration test.
- `examples/chatroom/client/main.go`: client CLI entrypoint and inbound print handler.

**Modify:**

- No framework core files.
- No existing codec packages.
- No existing transport, engine, channel, server, or client packages.

**Primary commands:**

- `go test ./examples/chatroom/... -count=1 -v`
- `go test ./... -count=1`
- `go run ./examples/chatroom/server -addr 127.0.0.1:8888`
- `go run ./examples/chatroom/client -addr 127.0.0.1:8888`

## Task 1: Shared Chat Message Package

**Files:**

- Create: `examples/chatroom/chat/message.go`
- Create: `examples/chatroom/chat/message_test.go`

- [ ] **Step 1: Write the failing shared package tests**

Create `examples/chatroom/chat/message_test.go`:

```go
package chat

import (
	"reflect"
	"testing"
)

func TestMessageConstructors(t *testing.T) {
	tests := []struct {
		name string
		got  *Message
		want *Message
	}{
		{
			name: "set name",
			got:  SetName("alice"),
			want: &Message{Type: TypeSetName, Name: "alice"},
		},
		{
			name: "chat",
			got:  Chat("alice", "hello"),
			want: &Message{Type: TypeChat, Name: "alice", Text: "hello"},
		},
		{
			name: "system",
			got:  System("alice joined"),
			want: &Message{Type: TypeSystem, Text: "alice joined"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("message = %#v, want %#v", tt.got, tt.want)
			}
		})
	}
}

func TestJSONCodecRoundTrip(t *testing.T) {
	codec := NewJSONCodec()

	frame, err := codec.Marshal(Chat("alice", "hello"))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	defer frame.Release()

	got, err := codec.Unmarshal(frame)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := &Message{Type: TypeChat, Name: "alice", Text: "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unmarshal() = %#v, want %#v", got, want)
	}
}
```

- [ ] **Step 2: Run the shared package tests and verify they fail**

Run:

```shell
go test ./examples/chatroom/chat -count=1 -v
```

Expected: FAIL because `examples/chatroom/chat` has no implementation package yet.

- [ ] **Step 3: Implement the shared message package**

Create `examples/chatroom/chat/message.go`:

```go
package chat

import (
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/payload"
)

const (
	TypeSetName = "set_name"
	TypeChat    = "chat"
	TypeSystem  = "system"
)

type Message struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	Text string `json:"text,omitempty"`
}

func SetName(name string) *Message {
	return &Message{Type: TypeSetName, Name: name}
}

func Chat(name, text string) *Message {
	return &Message{Type: TypeChat, Name: name, Text: text}
}

func System(text string) *Message {
	return &Message{Type: TypeSystem, Text: text}
}

func NewJSONCodec() codec.PayloadCodec {
	return payload.NewJSONCodecWithType(Message{})
}
```

- [ ] **Step 4: Run the shared package tests and verify they pass**

Run:

```shell
go test ./examples/chatroom/chat -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit the shared package**

Run:

```shell
git add examples/chatroom/chat/message.go examples/chatroom/chat/message_test.go
git commit -m "feat: add chatroom message package"
```

## Task 2: Server Hub And Handler Unit Tests

**Files:**

- Create: `examples/chatroom/server/hub.go`
- Create: `examples/chatroom/server/handlers.go`
- Create: `examples/chatroom/server/server_test.go`

- [ ] **Step 1: Write focused hub and handler tests**

Create `examples/chatroom/server/server_test.go` with the unit test section first:

```go
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

func (m *mockChannel) Context() context.Context { return context.Background() }
func (m *mockChannel) RemoteAddr() net.Addr     { return mockAddr{} }
func (m *mockChannel) LocalAddr() net.Addr      { return mockAddr{} }
func (m *mockChannel) IsActive() bool           { return !m.closed }
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

	tests := []struct {
		name    string
		message *chat.Message
		wantErr bool
	}{
		{name: "set name", message: chat.SetName("alice")},
		{name: "chat", message: chat.Chat("", "hello")},
		{name: "unknown", message: &chat.Message{Type: "unknown"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := router(context.Background(), &mockChannel{}, tt.message)
			if tt.wantErr && err == nil {
				t.Fatal("router() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("router() error = %v", err)
			}
		})
	}
}
```

- [ ] **Step 2: Run the server package tests and verify they fail**

Run:

```shell
go test ./examples/chatroom/server -count=1 -v
```

Expected: FAIL because `newHub`, `setName`, `broadcast`, `unregister`, `chatHandler`, and `newRouter` are undefined.

- [ ] **Step 3: Implement the server hub**

Create `examples/chatroom/server/hub.go`:

```go
package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/emove/less"
	"github.com/emove/less/examples/chatroom/chat"
)

type session struct {
	id      uint64
	name    string
	channel less.Channel
}

type hub struct {
	mu       sync.RWMutex
	nextID   uint64
	sessions map[less.Channel]*session
}

func newHub() *hub {
	return &hub{sessions: make(map[less.Channel]*session)}
}

func (h *hub) register(ch less.Channel) {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := atomic.AddUint64(&h.nextID, 1)
	h.sessions[ch] = &session{id: id, channel: ch}
}

func (h *hub) unregister(ch less.Channel) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sess, ok := h.sessions[ch]
	if !ok {
		return "", false
	}
	delete(h.sessions, ch)
	return sess.name, sess.name != ""
}

func (h *hub) setName(ch less.Channel, name string) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	sess, ok := h.sessions[ch]
	if !ok {
		id := atomic.AddUint64(&h.nextID, 1)
		sess = &session{id: id, channel: ch}
		h.sessions[ch] = sess
	}
	sess.name = strings.TrimSpace(name)
	return sess.name
}

func (h *hub) nameOf(ch less.Channel) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sess, ok := h.sessions[ch]
	if !ok || sess.name == "" {
		return "", false
	}
	return sess.name, true
}

func (h *hub) broadcast(msg *chat.Message) {
	for _, ch := range h.channels() {
		if err := ch.Write(msg); err != nil {
			ch.Close(fmt.Errorf("broadcast write failed: %w", err))
		}
	}
}

func (h *hub) channels() []less.Channel {
	h.mu.RLock()
	defer h.mu.RUnlock()

	channels := make([]less.Channel, 0, len(h.sessions))
	for ch := range h.sessions {
		channels = append(channels, ch)
	}
	return channels
}
```

- [ ] **Step 4: Implement server handlers and router**

Create `examples/chatroom/server/handlers.go`:

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/emove/less"
	"github.com/emove/less/examples/chatroom/chat"
)

func onChannel(h *hub) less.OnChannel {
	return func(ctx context.Context, ch less.Channel) (context.Context, error) {
		h.register(ch)
		return ctx, nil
	}
}

func onChannelClosed(h *hub) less.OnChannelClosed {
	return func(_ context.Context, ch less.Channel, _ error) {
		name, joined := h.unregister(ch)
		if joined {
			h.broadcast(chat.System(fmt.Sprintf("%s left", name)))
		}
	}
}

func newRouter(h *hub) less.Router {
	return func(_ context.Context, ch less.Channel, msg interface{}) (less.Handler, error) {
		message, ok := msg.(*chat.Message)
		if !ok {
			return nil, errors.New("chatroom message has unexpected type")
		}

		switch message.Type {
		case chat.TypeSetName:
			return setNameHandler(h), nil
		case chat.TypeChat:
			return chatHandler(h), nil
		default:
			_ = ch.Write(chat.System("unknown message type: " + message.Type))
			return nil, fmt.Errorf("unknown message type: %s", message.Type)
		}
	}
}

func setNameHandler(h *hub) less.Handler {
	return func(_ context.Context, ch less.Channel, msg interface{}) error {
		message := msg.(*chat.Message)
		name := strings.TrimSpace(message.Name)
		if name == "" {
			return ch.Write(chat.System("name cannot be empty"))
		}

		name = h.setName(ch, name)
		h.broadcast(chat.System(fmt.Sprintf("%s joined", name)))
		return nil
	}
}

func chatHandler(h *hub) less.Handler {
	return func(_ context.Context, ch less.Channel, msg interface{}) error {
		name, ok := h.nameOf(ch)
		if !ok {
			return ch.Write(chat.System("set a name before chatting"))
		}

		message := msg.(*chat.Message)
		text := strings.TrimSpace(message.Text)
		if text == "" {
			return ch.Write(chat.System("message cannot be empty"))
		}

		h.broadcast(chat.Chat(name, text))
		return nil
	}
}
```

- [ ] **Step 5: Run server tests and verify unit tests pass**

Run:

```shell
go test ./examples/chatroom/server -count=1 -v
```

Expected: PASS for the hub and handler unit tests.

- [ ] **Step 6: Commit server hub and handlers**

Run:

```shell
git add examples/chatroom/server/hub.go examples/chatroom/server/handlers.go examples/chatroom/server/server_test.go
git commit -m "feat: add chatroom server hub"
```

## Task 3: Server Entrypoint

**Files:**

- Create: `examples/chatroom/server/main.go`
- Modify: `examples/chatroom/server/server_test.go`

- [ ] **Step 1: Add a server construction smoke test**

Append this test to `examples/chatroom/server/server_test.go`:

```go
func TestNewChatServer(t *testing.T) {
	srv := newChatServer("127.0.0.1:0", newHub())
	if srv == nil {
		t.Fatal("newChatServer() = nil, want server")
	}
}
```

- [ ] **Step 2: Run the server package tests and verify they fail**

Run:

```shell
go test ./examples/chatroom/server -count=1 -v
```

Expected: FAIL because `newChatServer` is undefined.

- [ ] **Step 3: Implement the server entrypoint**

Create `examples/chatroom/server/main.go`:

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/emove/less/codec/packet"
	"github.com/emove/less/examples/chatroom/chat"
	"github.com/emove/less/server"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8888", "chatroom listen address")
	flag.Parse()

	h := newHub()
	srv := newChatServer(*addr, h)
	srv.Run()
	fmt.Printf("chatroom server listening on %s\n", *addr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	srv.Shutdown(context.Background(), nil)
}

func newChatServer(addr string, h *hub) *server.Server {
	return server.NewServer(
		addr,
		server.WithPacketCodec(packet.NewVariableLengthCodec()),
		server.WithPayloadCodec(chat.NewJSONCodec()),
		server.WithOnChannel(onChannel(h)),
		server.WithOnChannelClosed(onChannelClosed(h)),
		server.WithRouter(newRouter(h)),
	)
}
```

- [ ] **Step 4: Run the server package tests and verify they pass**

Run:

```shell
go test ./examples/chatroom/server -count=1 -v
```

Expected: PASS.

- [ ] **Step 5: Commit the server entrypoint**

Run:

```shell
git add examples/chatroom/server/main.go examples/chatroom/server/server_test.go
git commit -m "feat: add chatroom server entrypoint"
```

## Task 4: Client Entrypoint

**Files:**

- Create: `examples/chatroom/client/main.go`

- [ ] **Step 1: Write the client program**

Create `examples/chatroom/client/main.go`:

```go
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/examples/chatroom/chat"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8888", "chatroom server address")
	flag.Parse()

	input := bufio.NewScanner(os.Stdin)
	fmt.Print("name: ")
	if !input.Scan() {
		return
	}
	name := strings.TrimSpace(input.Text())
	if name == "" {
		fmt.Fprintln(os.Stderr, "name cannot be empty")
		os.Exit(1)
	}

	cli := newChatClient(*addr)
	defer cli.Close(nil)

	if err := cli.Dial(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "dial failed: %v\n", err)
		os.Exit(1)
	}

	ch := cli.Channel()
	if ch == nil {
		fmt.Fprintln(os.Stderr, "dial succeeded without an active channel")
		os.Exit(1)
	}

	if err := ch.Write(chat.SetName(name)); err != nil {
		fmt.Fprintf(os.Stderr, "set name failed: %v\n", err)
		os.Exit(1)
	}

	for input.Scan() {
		text := strings.TrimSpace(input.Text())
		if text == "" {
			continue
		}
		if err := ch.Write(chat.Chat("", text)); err != nil {
			fmt.Fprintf(os.Stderr, "send failed: %v\n", err)
			return
		}
	}
}

func newChatClient(addr string) *client.Client {
	return client.NewClient(
		"tcp",
		addr,
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(chat.NewJSONCodec()),
		client.WithRouter(printRouter()),
	)
}

func printRouter() less.Router {
	return func(_ context.Context, _ less.Channel, msg interface{}) (less.Handler, error) {
		return printHandler, nil
	}
}

func printHandler(_ context.Context, _ less.Channel, msg interface{}) error {
	message, ok := msg.(*chat.Message)
	if !ok {
		return fmt.Errorf("unexpected message type %T", msg)
	}

	switch message.Type {
	case chat.TypeSystem:
		fmt.Printf("[system] %s\n", message.Text)
	case chat.TypeChat:
		fmt.Printf("[%s] %s\n", message.Name, message.Text)
	default:
		fmt.Printf("[system] unknown message type: %s\n", message.Type)
	}
	return nil
}
```

- [ ] **Step 2: Run chatroom example package tests**

Run:

```shell
go test ./examples/chatroom/... -count=1 -v
```

Expected: PASS.

- [ ] **Step 3: Commit the client entrypoint**

Run:

```shell
git add examples/chatroom/client/main.go
git commit -m "feat: add chatroom client entrypoint"
```

## Task 5: Two-Client Integration Test

**Files:**

- Modify: `examples/chatroom/server/server_test.go`

- [ ] **Step 1: Add the integration test helpers and scenario**

Append this integration test section to `examples/chatroom/server/server_test.go`:

```go
const defaultWaitTimeout = 3 * time.Second

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

func receiveMessage(t *testing.T, messages <-chan *chat.Message, description string) *chat.Message {
	t.Helper()

	select {
	case msg := <-messages:
		return msg
	case <-time.After(defaultWaitTimeout):
		t.Fatalf("timed out waiting for %s", description)
		return nil
	}
}

func newTestClient(t *testing.T, addr string, inbound chan<- *chat.Message) *client.Client {
	t.Helper()

	return client.NewClient(
		"tcp",
		addr,
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(chat.NewJSONCodec()),
		client.WithRouter(func(context.Context, less.Channel, any) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, msg any) error {
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

func dialClientEventually(t *testing.T, cli *client.Client) {
	t.Helper()

	deadline := time.Now().Add(defaultWaitTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := cli.Dial(context.Background()); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client dial did not succeed before deadline: %v", lastErr)
}

func TestChatroomTwoClientsBroadcastAndLeave(t *testing.T) {
	addr := reserveTCPAddr(t)
	h := newHub()
	srv := newChatServer(addr, h)
	srv.Run()
	t.Cleanup(func() {
		srv.Shutdown(context.Background(), nil)
	})

	aliceMessages := make(chan *chat.Message, 8)
	bobMessages := make(chan *chat.Message, 8)
	alice := newTestClient(t, addr, aliceMessages)
	bob := newTestClient(t, addr, bobMessages)
	t.Cleanup(func() {
		alice.Close(nil)
		bob.Close(nil)
	})

	dialClientEventually(t, alice)
	dialClientEventually(t, bob)

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

	var gotChat *chat.Message
	for i := 0; i < 4; i++ {
		msg := receiveMessage(t, bobMessages, "bob chat broadcast")
		if msg.Type == chat.TypeChat {
			gotChat = msg
			break
		}
	}
	if gotChat == nil {
		t.Fatal("bob did not receive a chat broadcast")
	}
	wantChat := chat.Chat("alice", "hello")
	if !reflect.DeepEqual(gotChat, wantChat) {
		t.Fatalf("bob chat = %#v, want %#v", gotChat, wantChat)
	}

	alice.Close(nil)

	var gotLeave *chat.Message
	for i := 0; i < 4; i++ {
		msg := receiveMessage(t, bobMessages, "bob leave message")
		if msg.Type == chat.TypeSystem && msg.Text == "alice left" {
			gotLeave = msg
			break
		}
	}
	if gotLeave == nil {
		t.Fatal("bob did not receive alice leave message")
	}
}
```

Update the import block in `examples/chatroom/server/server_test.go` to include every package used by the unit and integration tests:

```go
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
```

- [ ] **Step 2: Run the server integration test**

Run:

```shell
go test ./examples/chatroom/server -count=1 -run TestChatroomTwoClientsBroadcastAndLeave -v
```

Expected: PASS.

- [ ] **Step 3: Run all chatroom example tests**

Run:

```shell
go test ./examples/chatroom/... -count=1 -v
```

Expected: PASS.

- [ ] **Step 4: Commit the integration test**

Run:

```shell
git add examples/chatroom/server/server_test.go
git commit -m "test: cover chatroom two-client flow"
```

## Task 6: Full Verification And Manual Smoke Test

**Files:**

- Modify only files needed to fix failures found by verification.

- [ ] **Step 1: Format the example code**

Run:

```shell
gofmt -w examples/chatroom
```

Expected: command exits 0.

- [ ] **Step 2: Run focused tests**

Run:

```shell
go test ./examples/chatroom/... -count=1 -v
```

Expected: PASS.

- [ ] **Step 3: Run full repository tests**

Run:

```shell
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Manually smoke test the runnable server startup**

Run:

```shell
go run ./examples/chatroom/server -addr 127.0.0.1:18888
```

Expected: process prints:

```text
chatroom server listening on 127.0.0.1:18888
```

Stop the process with `Ctrl-C`.

- [ ] **Step 5: Commit any verification fixes**

If formatting or verification changed files, run:

```shell
git add examples/chatroom
git commit -m "chore: verify chatroom example"
```

If no files changed, do not create an empty commit.

## Self-Review

- Spec coverage: The plan creates `examples/chatroom/server`, `examples/chatroom/client`, and `examples/chatroom/chat`; uses `less/server`, `less/client`, `OnChannel`, `OnChannelClosed`, `Router`, `Channel.Write`, JSON payloads, and length-prefixed packets; adds a two-client integration test; avoids WebSocket, framework core changes, fixed test ports, and third-party dependencies.
- Placeholder scan: No task uses open-ended placeholder wording. Each code-producing task names exact files and includes concrete code.
- Type consistency: The plan consistently uses `*chat.Message`, `chat.TypeSetName`, `chat.TypeChat`, `chat.TypeSystem`, `chat.NewJSONCodec()`, `newHub()`, `newChatServer()`, `onChannel()`, `onChannelClosed()`, `newRouter()`, `setNameHandler()`, and `chatHandler()`.
