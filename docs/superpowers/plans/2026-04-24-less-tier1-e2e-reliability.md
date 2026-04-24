# Less Tier 1 End-to-End Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CI-default Tier 1 end-to-end reliability tests for real `server` to `client` behavior through public APIs only.

**Architecture:** Create a new external test package under `test/e2e` so the tests can import only public packages and exercise the same surface a framework user sees. Build a small test harness for dynamic TCP addresses, dial readiness, lifecycle counters, and deadline-based waits, then add focused scenarios for lifecycle, concurrency, shutdown, frame boundaries, and malformed input.

**Tech Stack:** Go 1.19, standard library `testing`/`net`/`context`/`sync`/`sync/atomic`/`time`, existing public packages `less`, `client`, `server`, `codec/packet`, and `codec/payload`

---

## File Structure

**Create:**

- `test/e2e/e2e_test.go`: external-package Tier 1 e2e tests and their small local harness.

**Modify:**

- No production files should be modified while adding the tests.
- Production files should only be touched if a new e2e test exposes a real contract bug.

**Primary commands:**

- `go test ./test/e2e -count=1 -v`
- `go test ./... -count=1`

## Task 1: Create the e2e harness and full lifecycle test

**Files:**

- Create: `test/e2e/e2e_test.go`
- Test: `test/e2e/e2e_test.go`

- [ ] **Step 1: Write the external-package harness and full lifecycle test**

Create `test/e2e/e2e_test.go` with this content:

```go
package e2e_test

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/server"
)

const (
	defaultWaitTimeout = 3 * time.Second
	packetHeaderSize   = binary.MaxVarintLen32
)

type eventCounter struct {
	count int32
	ch    chan struct{}
}

func newEventCounter(capacity int) *eventCounter {
	if capacity < 1 {
		capacity = 1
	}
	return &eventCounter{ch: make(chan struct{}, capacity)}
}

func (c *eventCounter) inc() {
	atomic.AddInt32(&c.count, 1)
	select {
	case c.ch <- struct{}{}:
	default:
	}
}

func (c *eventCounter) value() int32 {
	return atomic.LoadInt32(&c.count)
}

func (c *eventCounter) waitFor(t *testing.T, want int32) {
	t.Helper()

	deadline := time.Now().Add(defaultWaitTimeout)
	for time.Now().Before(deadline) {
		if c.value() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := c.value(); got < want {
		t.Fatalf("event count reached %d, want at least %d", got, want)
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

func waitUntil(t *testing.T, timeout time.Duration, predicate func() bool, failureFormat string, args ...any) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	if predicate() {
		return
	}
	t.Fatalf(failureFormat, args...)
}

func dialClientEventually(t *testing.T, cli *client.Client) {
	t.Helper()
	if err := dialClientWithTimeout(cli); err != nil {
		t.Fatalf("client dial did not succeed before deadline: %v", err)
	}
}

func dialClientWithTimeout(cli *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultWaitTimeout)
	defer cancel()

	var lastErr error
	for ctx.Err() == nil {
		err := cli.Dial(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	if lastErr != nil {
		return lastErr
	}
	return ctx.Err()
}

func newTextClient(addr string, opts ...client.CliOption) *client.Client {
	base := []client.CliOption{
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(payload.NewTextCodec()),
	}
	base = append(base, opts...)
	return client.NewClient("tcp", addr, base...)
}

func newTextServer(addr string, opts ...server.SerOption) *server.Server {
	base := []server.SerOption{
		server.WithPacketCodec(packet.NewVariableLengthCodec()),
		server.WithPayloadCodec(payload.NewTextCodec()),
	}
	base = append(base, opts...)
	return server.NewServer(addr, base...)
}

func TestTier1E2E_FullLifecycle(t *testing.T) {
	addr := reserveTCPAddr(t)

	serverOnChannel := newEventCounter(1)
	serverClosed := newEventCounter(1)
	clientOnChannel := newEventCounter(1)
	clientClosed := newEventCounter(1)

	serverChannelReady := make(chan less.Channel, 1)
	serverReceived := make(chan string, 1)
	clientReceived := make(chan string, 1)

	srv := newTextServer(
		addr,
		server.WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			serverOnChannel.inc()
			select {
			case serverChannelReady <- ch:
			default:
			}
			return ctx, nil
		}),
		server.WithOnChannelClosed(func(context.Context, less.Channel, error) {
			serverClosed.inc()
		}),
		server.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, message any) error {
				serverReceived <- message.(string)
				return nil
			}, nil
		}),
	)
	srv.Run()
	t.Cleanup(func() {
		srv.Shutdown(context.Background(), nil)
	})

	cli := newTextClient(
		addr,
		client.WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			clientOnChannel.inc()
			return ctx, nil
		}),
		client.WithOnChannelClosed(func(context.Context, less.Channel, error) {
			clientClosed.inc()
		}),
		client.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, message any) error {
				clientReceived <- message.(string)
				return nil
			}, nil
		}),
	)
	t.Cleanup(func() {
		cli.Close(nil)
	})

	dialClientEventually(t, cli)
	serverOnChannel.waitFor(t, 1)
	clientOnChannel.waitFor(t, 1)

	if cli.Channel() == nil {
		t.Fatal("expected client channel to be active after Dial")
	}

	var serverChannel less.Channel
	select {
	case serverChannel = <-serverChannelReady:
	case <-time.After(defaultWaitTimeout):
		t.Fatal("server OnChannel did not publish a channel")
	}
	if serverChannel == nil || !serverChannel.IsActive() {
		t.Fatal("expected server channel to be active after client Dial")
	}

	if err := cli.Channel().Write("client-to-server"); err != nil {
		t.Fatalf("client Write failed: %v", err)
	}
	select {
	case got := <-serverReceived:
		if got != "client-to-server" {
			t.Fatalf("server received %q, want %q", got, "client-to-server")
		}
	case <-time.After(defaultWaitTimeout):
		t.Fatal("server did not receive client message")
	}

	if err := serverChannel.Write("server-to-client"); err != nil {
		t.Fatalf("server Write failed: %v", err)
	}
	select {
	case got := <-clientReceived:
		if got != "server-to-client" {
			t.Fatalf("client received %q, want %q", got, "server-to-client")
		}
	case <-time.After(defaultWaitTimeout):
		t.Fatal("client did not receive server message")
	}

	cli.Close(errors.New("client requested close"))
	serverClosed.waitFor(t, 1)
	clientClosed.waitFor(t, 1)

	waitUntil(t, defaultWaitTimeout, func() bool {
		return cli.Channel() == nil
	}, "client channel remained active after Close")

	if got := serverOnChannel.value(); got != 1 {
		t.Fatalf("server OnChannel count = %d, want 1", got)
	}
	if got := clientOnChannel.value(); got != 1 {
		t.Fatalf("client OnChannel count = %d, want 1", got)
	}
	if got := serverClosed.value(); got != 1 {
		t.Fatalf("server OnChannelClosed count = %d, want 1", got)
	}
	if got := clientClosed.value(); got != 1 {
		t.Fatalf("client OnChannelClosed count = %d, want 1", got)
	}
}
```

- [ ] **Step 2: Run the new e2e package**

Run: `go test ./test/e2e -run TestTier1E2E_FullLifecycle -count=1 -v`

Expected: PASS. If it fails, inspect the failure as a real lifecycle contract issue before adding more scenarios.

- [ ] **Step 3: Run all tests**

Run: `go test ./... -count=1`

Expected: PASS, including the new `github.com/emove/less/test/e2e` package.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/e2e_test.go
git commit -m "test: add tier1 e2e lifecycle coverage"
```

## Task 2: Add concurrent client coverage

**Files:**

- Modify: `test/e2e/e2e_test.go`
- Test: `test/e2e/e2e_test.go`

- [ ] **Step 1: Add the concurrent clients test**

Update the import block in `test/e2e/e2e_test.go` to include the packages used by the concurrent test:

```go
import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/server"
)
```

Append this test to `test/e2e/e2e_test.go`:

```go
func TestTier1E2E_ConcurrentClients(t *testing.T) {
	addr := reserveTCPAddr(t)

	const clientCount = 8
	const messagesPerClient = 5
	wantMessages := int32(clientCount * messagesPerClient)

	serverOnChannel := newEventCounter(clientCount)
	serverClosed := newEventCounter(clientCount)
	var serverReceived int32

	srv := newTextServer(
		addr,
		server.WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			serverOnChannel.inc()
			return ctx, nil
		}),
		server.WithOnChannelClosed(func(context.Context, less.Channel, error) {
			serverClosed.inc()
		}),
		server.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
			return func(_ context.Context, channel less.Channel, message any) error {
				atomic.AddInt32(&serverReceived, 1)
				return channel.Write("ack:" + message.(string))
			}, nil
		}),
	)
	srv.Run()
	t.Cleanup(func() {
		srv.Shutdown(context.Background(), nil)
	})

	var wg sync.WaitGroup
	errs := make(chan error, clientCount)
	for clientID := 0; clientID < clientCount; clientID++ {
		clientID := clientID
		wg.Add(1)
		go func() {
			defer wg.Done()

			acks := make(chan string, messagesPerClient)
			cli := newTextClient(
				addr,
				client.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
					return func(_ context.Context, _ less.Channel, message any) error {
						acks <- message.(string)
						return nil
					}, nil
				}),
			)
			defer cli.Close(nil)

			if err := dialClientWithTimeout(cli); err != nil {
				errs <- fmt.Errorf("client %d dial failed: %w", clientID, err)
				return
			}
			ch := cli.Channel()
			if ch == nil {
				errs <- fmt.Errorf("client %d did not capture a channel", clientID)
				return
			}

			for seq := 0; seq < messagesPerClient; seq++ {
				msg := fmt.Sprintf("client=%d seq=%d", clientID, seq)
				if err := ch.Write(msg); err != nil {
					errs <- fmt.Errorf("client %d write %d failed: %w", clientID, seq, err)
					return
				}
				select {
				case got := <-acks:
					want := "ack:" + msg
					if got != want {
						errs <- fmt.Errorf("client %d ack %d = %q, want %q", clientID, seq, got, want)
						return
					}
				case <-time.After(defaultWaitTimeout):
					errs <- fmt.Errorf("client %d timed out waiting for ack %d", clientID, seq)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	serverOnChannel.waitFor(t, clientCount)
	waitUntil(t, defaultWaitTimeout, func() bool {
		return atomic.LoadInt32(&serverReceived) == wantMessages
	}, "server received %d messages, want %d", atomic.LoadInt32(&serverReceived), wantMessages)

	serverClosed.waitFor(t, clientCount)
	if got := serverClosed.value(); got != clientCount {
		t.Fatalf("server close count = %d, want %d", got, clientCount)
	}
}
```

- [ ] **Step 2: Run the concurrent clients test**

Run: `go test ./test/e2e -run TestTier1E2E_ConcurrentClients -count=1 -v`

Expected: PASS in under 8 seconds.

- [ ] **Step 3: Run the e2e package**

Run: `go test ./test/e2e -count=1 -v`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/e2e_test.go
git commit -m "test: cover concurrent tier1 e2e clients"
```

## Task 3: Add server shutdown coverage for active clients

**Files:**

- Modify: `test/e2e/e2e_test.go`
- Test: `test/e2e/e2e_test.go`

- [ ] **Step 1: Add the active-client shutdown test**

Append this test to `test/e2e/e2e_test.go`:

```go
func TestTier1E2E_ServerShutdownClosesActiveClients(t *testing.T) {
	addr := reserveTCPAddr(t)

	const clientCount = 4

	serverOnChannel := newEventCounter(clientCount)
	serverClosed := newEventCounter(clientCount)
	clientClosed := newEventCounter(clientCount)

	srv := newTextServer(
		addr,
		server.WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			serverOnChannel.inc()
			return ctx, nil
		}),
		server.WithOnChannelClosed(func(context.Context, less.Channel, error) {
			serverClosed.inc()
		}),
		server.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
			return func(context.Context, less.Channel, any) error {
				return nil
			}, nil
		}),
	)
	srv.Run()

	clients := make([]*client.Client, 0, clientCount)
	for i := 0; i < clientCount; i++ {
		cli := newTextClient(
			addr,
			client.WithOnChannelClosed(func(context.Context, less.Channel, error) {
				clientClosed.inc()
			}),
			client.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
				return func(context.Context, less.Channel, any) error {
					return nil
				}, nil
			}),
		)
		dialClientEventually(t, cli)
		clients = append(clients, cli)
	}
	t.Cleanup(func() {
		for _, cli := range clients {
			cli.Close(nil)
		}
		srv.Shutdown(context.Background(), nil)
	})

	serverOnChannel.waitFor(t, clientCount)

	srv.Shutdown(context.Background(), errors.New("test shutdown"))

	serverClosed.waitFor(t, clientCount)
	clientClosed.waitFor(t, clientCount)

	for i, cli := range clients {
		waitUntil(t, defaultWaitTimeout, func() bool {
			return cli.Channel() == nil
		}, "client %d channel remained active after server shutdown", i)
	}

	if got := serverClosed.value(); got != clientCount {
		t.Fatalf("server close count = %d, want %d", got, clientCount)
	}
	if got := clientClosed.value(); got != clientCount {
		t.Fatalf("client close count = %d, want %d", got, clientCount)
	}
}
```

- [ ] **Step 2: Run the shutdown test**

Run: `go test ./test/e2e -run TestTier1E2E_ServerShutdownClosesActiveClients -count=1 -v`

Expected: PASS in under 3 seconds.

- [ ] **Step 3: Run all tests**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/e2e_test.go
git commit -m "test: cover tier1 server shutdown with active clients"
```

## Task 4: Add codec and frame boundary coverage

**Files:**

- Modify: `test/e2e/e2e_test.go`
- Test: `test/e2e/e2e_test.go`

- [ ] **Step 1: Add the codec and frame boundary test**

Update the import block in `test/e2e/e2e_test.go` to include `strings`:

```go
import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/server"
)
```

Append this test to `test/e2e/e2e_test.go`:

```go
func TestTier1E2E_CodecAndFrameBoundaries(t *testing.T) {
	addr := reserveTCPAddr(t)

	serverReceived := make(chan string, 16)
	clientReceived := make(chan string, 16)
	serverChannelReady := make(chan less.Channel, 1)

	srv := newTextServer(
		addr,
		server.WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			select {
			case serverChannelReady <- ch:
			default:
			}
			return ctx, nil
		}),
		server.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, message any) error {
				serverReceived <- message.(string)
				return nil
			}, nil
		}),
	)
	srv.Run()
	t.Cleanup(func() {
		srv.Shutdown(context.Background(), nil)
	})

	cli := newTextClient(
		addr,
		client.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, message any) error {
				clientReceived <- message.(string)
				return nil
			}, nil
		}),
	)
	t.Cleanup(func() {
		cli.Close(nil)
	})

	dialClientEventually(t, cli)
	if cli.Channel() == nil {
		t.Fatal("expected client channel after Dial")
	}

	var serverChannel less.Channel
	select {
	case serverChannel = <-serverChannelReady:
	case <-time.After(defaultWaitTimeout):
		t.Fatal("server channel was not published")
	}

	payloads := []string{"ping"}
	for i := 0; i < 10; i++ {
		payloads = append(payloads, fmt.Sprintf("burst-%02d", i))
	}
	payloads = append(payloads, strings.Repeat("x", 64*1024))

	for i, payload := range payloads {
		if err := cli.Channel().Write(payload); err != nil {
			t.Fatalf("client write payload %d failed: %v", i, err)
		}
		select {
		case got := <-serverReceived:
			if got != payload {
				t.Fatalf("server payload %d length = %d, want %d", i, len(got), len(payload))
			}
		case <-time.After(defaultWaitTimeout):
			t.Fatalf("server timed out waiting for payload %d", i)
		}
	}

	for i, payload := range payloads {
		reply := "reply:" + payload
		if err := serverChannel.Write(reply); err != nil {
			t.Fatalf("server write payload %d failed: %v", i, err)
		}
		select {
		case got := <-clientReceived:
			if got != reply {
				t.Fatalf("client payload %d length = %d, want %d", i, len(got), len(reply))
			}
		case <-time.After(defaultWaitTimeout):
			t.Fatalf("client timed out waiting for payload %d", i)
		}
	}
}
```

- [ ] **Step 2: Run the codec and frame boundary test**

Run: `go test ./test/e2e -run TestTier1E2E_CodecAndFrameBoundaries -count=1 -v`

Expected: PASS in under 3 seconds.

- [ ] **Step 3: Run the e2e package**

Run: `go test ./test/e2e -count=1 -v`

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/e2e_test.go
git commit -m "test: cover tier1 codec frame boundaries"
```

## Task 5: Add malformed packet failure-path coverage

**Files:**

- Modify: `test/e2e/e2e_test.go`
- Test: `test/e2e/e2e_test.go`

- [ ] **Step 1: Add raw packet helper functions**

Append these helpers to `test/e2e/e2e_test.go`:

```go
func writeRawVariableLengthPacket(t *testing.T, conn net.Conn, declaredLength uint32, body string) {
	t.Helper()

	header := make([]byte, packetHeaderSize)
	binary.BigEndian.PutUint32(header, declaredLength)
	if _, err := conn.Write(header); err != nil {
		t.Fatalf("write raw packet header: %v", err)
	}
	if body != "" {
		if _, err := conn.Write([]byte(body)); err != nil {
			t.Fatalf("write raw packet body: %v", err)
		}
	}
}

func dialRawEventually(t *testing.T, addr string) net.Conn {
	t.Helper()

	deadline := time.Now().Add(defaultWaitTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			return conn
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("raw dial did not succeed: %v", lastErr)
	return nil
}
```

- [ ] **Step 2: Add the malformed packet test**

Append this test to `test/e2e/e2e_test.go`:

```go
func TestTier1E2E_MalformedPacketClosesConnectionOnce(t *testing.T) {
	addr := reserveTCPAddr(t)

	serverOnChannel := newEventCounter(1)
	serverClosed := newEventCounter(1)
	var handlerCalls int32

	srv := newTextServer(
		addr,
		server.WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			serverOnChannel.inc()
			return ctx, nil
		}),
		server.WithOnChannelClosed(func(context.Context, less.Channel, error) {
			serverClosed.inc()
		}),
		server.WithRouter(func(ctx context.Context, ch less.Channel, msg any) (less.Handler, error) {
			return func(context.Context, less.Channel, any) error {
				atomic.AddInt32(&handlerCalls, 1)
				return nil
			}, nil
		}),
	)
	srv.Run()
	t.Cleanup(func() {
		srv.Shutdown(context.Background(), nil)
	})

	conn := dialRawEventually(t, addr)
	writeRawVariableLengthPacket(t, conn, 32, "short")
	_ = conn.Close()

	serverOnChannel.waitFor(t, 1)
	serverClosed.waitFor(t, 1)

	if got := serverClosed.value(); got != 1 {
		t.Fatalf("server close count = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&handlerCalls); got != 0 {
		t.Fatalf("handler calls after malformed packet = %d, want 0", got)
	}
}
```

- [ ] **Step 3: Run the malformed packet test**

Run: `go test ./test/e2e -run TestTier1E2E_MalformedPacketClosesConnectionOnce -count=1 -v`

Expected: PASS. If it hangs or handler calls become nonzero, treat that as a failure-path bug.

- [ ] **Step 4: Run all tests**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add test/e2e/e2e_test.go
git commit -m "test: cover tier1 malformed packet close path"
```

## Task 6: Verify CI budget and external-package boundaries

**Files:**

- Modify: `test/e2e/e2e_test.go` only if verification exposes test flakiness.
- Test: `test/e2e/e2e_test.go`

- [ ] **Step 1: Confirm the e2e file imports only public packages**

Run: `rg -n 'github.com/emove/less/internal|github.com/emove/less/internal/' test/e2e/e2e_test.go`

Expected: no output. If output includes `github.com/emove/less/internal/...`, remove that direct import and use only public packages.

- [ ] **Step 2: Check for fixed-port and sleep-as-success patterns**

Run: `rg -n '"localhost:8888"|":8888"|time\\.Sleep\\([^)]*\\).*success' test/e2e/e2e_test.go`

Expected: no output. The only allowed `time.Sleep` in this file is inside retry polling helpers, never as the success condition.

- [ ] **Step 3: Run the full e2e package with repeated count**

Run: `go test ./test/e2e -count=3 -v`

Expected: PASS. Use the elapsed time printed by Go to confirm this package stays comfortably below the 30 second Tier 1 budget.

- [ ] **Step 4: Run the full repository test suite**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 5: Run the race detector on the e2e package**

Run: `go test -race ./test/e2e -count=1`

Expected: PASS. If the race detector reports a data race in test-local state, protect that state with a mutex or atomic variable in `test/e2e/e2e_test.go`. If it reports a race in production code, stop and fix the production race with a focused test before continuing.

- [ ] **Step 6: Commit verification-only test cleanup if any file changed**

If `git status --short` shows modifications to `test/e2e/e2e_test.go`, commit them:

```bash
git add test/e2e/e2e_test.go
git commit -m "test: stabilize tier1 e2e reliability suite"
```

If no files changed, do not create an empty commit.

## Self-Review Checklist

- [ ] The plan creates tests only under `test/e2e/e2e_test.go`.
- [ ] The plan keeps tests on public APIs: `less`, `client`, `server`, `codec/packet`, `codec/payload`.
- [ ] The plan covers full lifecycle, concurrent clients, server shutdown, codec/frame boundaries, and malformed input.
- [ ] Every wait has a deadline.
- [ ] Tests avoid fixed ports.
- [ ] Tests avoid log-based assertions.
- [ ] `go test ./... -count=1` is the final repository-level verification.
- [ ] `go test -race ./test/e2e -count=1` is included as a reliability check.
