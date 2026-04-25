package main

import (
	"context"
	"errors"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/examples/device-gateway/protocol"
)

type mockAddr struct{}

func (mockAddr) Network() string { return "tcp" }
func (mockAddr) String() string  { return "127.0.0.1:9999" }

type mockChannel struct {
	mu       sync.Mutex
	writes   []any
	writeErr error
	closeErr error
	closed   bool
}

func (m *mockChannel) Context() context.Context                   { return context.Background() }
func (m *mockChannel) RemoteAddr() net.Addr                       { return mockAddr{} }
func (m *mockChannel) LocalAddr() net.Addr                        { return mockAddr{} }
func (m *mockChannel) IsActive() bool                             { return !m.closed }
func (m *mockChannel) AddOnChannelClosed(...less.OnChannelClosed) {}
func (m *mockChannel) AddInboundMiddleware(...less.Middleware)    {}
func (m *mockChannel) AddOutboundMiddleware(...less.Middleware)   {}

func (m *mockChannel) Write(msg interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeErr != nil {
		return m.writeErr
	}
	m.writes = append(m.writes, msg)
	return nil
}

func (m *mockChannel) Close(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.closeErr = err
}

func (m *mockChannel) snapshot() []any {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]any, len(m.writes))
	copy(out, m.writes)
	return out
}

func TestAuthHandlerMarksSessionAuthenticatedAndReplies(t *testing.T) {
	gw := newGateway(2 * time.Second)
	ch := &mockChannel{}
	if _, err := onChannel(gw)(context.Background(), ch); err != nil {
		t.Fatalf("onChannel() error = %v", err)
	}

	handler, err := newRouter(gw)(context.Background(), ch, protocol.Auth("dev-001", demoSecret))
	if err != nil {
		t.Fatalf("router() error = %v", err)
	}
	if err := handler(context.Background(), ch, protocol.Auth("dev-001", demoSecret)); err != nil {
		t.Fatalf("handler() error = %v", err)
	}

	session, ok := gw.registry.session(ch)
	if !ok {
		t.Fatal("session() ok = false, want true")
	}
	if !session.authenticated || session.deviceID != "dev-001" {
		t.Fatalf("session = %#v, want authenticated device dev-001", session)
	}

	want := []any{protocol.AuthAck("ok")}
	if got := ch.snapshot(); !reflect.DeepEqual(got, want) {
		t.Fatalf("writes = %#v, want %#v", got, want)
	}
}

func TestTelemetryHandlerWritesCommand(t *testing.T) {
	gw := newGateway(2 * time.Second)
	ch := &mockChannel{}
	if _, err := onChannel(gw)(context.Background(), ch); err != nil {
		t.Fatalf("onChannel() error = %v", err)
	}
	if err := authHandler(gw)(context.Background(), ch, protocol.Auth("dev-001", demoSecret)); err != nil {
		t.Fatalf("authHandler() error = %v", err)
	}

	if err := telemetryHandler(gw)(context.Background(), ch, protocol.Telemetry(map[string]string{"temp": "23.4"})); err != nil {
		t.Fatalf("telemetryHandler() error = %v", err)
	}

	writes := ch.snapshot()
	if len(writes) != 2 {
		t.Fatalf("write count = %d, want 2", len(writes))
	}
	cmd, ok := writes[1].(*protocol.CommandMessage)
	if !ok {
		t.Fatalf("second write = %T, want *protocol.CommandMessage", writes[1])
	}
	if cmd.RequestID == 0 || cmd.Name != "reboot" {
		t.Fatalf("command = %#v, want request id and reboot", cmd)
	}
}

func TestHeartbeatBeforeAuthReturnsError(t *testing.T) {
	gw := newGateway(2 * time.Second)
	ch := &mockChannel{}
	if _, err := onChannel(gw)(context.Background(), ch); err != nil {
		t.Fatalf("onChannel() error = %v", err)
	}

	err := heartbeatHandler(gw)(context.Background(), ch, protocol.Heartbeat(1714032000))
	if !errors.Is(err, errUnauthenticated) {
		t.Fatalf("heartbeatHandler() error = %v, want %v", err, errUnauthenticated)
	}
}

func TestOnChannelClosedRemovesSession(t *testing.T) {
	gw := newGateway(2 * time.Second)
	ch := &mockChannel{}
	if _, err := onChannel(gw)(context.Background(), ch); err != nil {
		t.Fatalf("onChannel() error = %v", err)
	}

	onChannelClosed(gw)(context.Background(), ch, nil)

	if _, ok := gw.registry.session(ch); ok {
		t.Fatal("session() ok = true, want false")
	}
}

func TestAuthHandlerFailedAckWriteDoesNotAuthenticateSession(t *testing.T) {
	gw := newGateway(2 * time.Second)
	ch := &mockChannel{writeErr: errors.New("write failed")}
	if _, err := onChannel(gw)(context.Background(), ch); err != nil {
		t.Fatalf("onChannel() error = %v", err)
	}

	err := authHandler(gw)(context.Background(), ch, protocol.Auth("dev-001", demoSecret))
	if err == nil {
		t.Fatal("authHandler() error = nil, want write failure")
	}

	session, ok := gw.registry.session(ch)
	if !ok {
		t.Fatal("session() ok = false, want true")
	}
	if session.authenticated {
		t.Fatalf("session = %#v, want unauthenticated after failed auth ack write", session)
	}
}

func TestHeartbeatUsesServerReceiveTimeInsteadOfDeviceTimestamp(t *testing.T) {
	gw := newGateway(2 * time.Second)
	ch := &mockChannel{}
	if _, err := onChannel(gw)(context.Background(), ch); err != nil {
		t.Fatalf("onChannel() error = %v", err)
	}
	if err := authHandler(gw)(context.Background(), ch, protocol.Auth("dev-001", demoSecret)); err != nil {
		t.Fatalf("authHandler() error = %v", err)
	}

	before := time.Now()
	futureTimestamp := time.Date(2500, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()
	if err := heartbeatHandler(gw)(context.Background(), ch, protocol.Heartbeat(futureTimestamp)); err != nil {
		t.Fatalf("heartbeatHandler() error = %v", err)
	}
	after := time.Now()

	session, ok := gw.registry.session(ch)
	if !ok {
		t.Fatal("session() ok = false, want true")
	}
	if session.lastHeartbeatAt.Before(before) || session.lastHeartbeatAt.After(after) {
		t.Fatalf("lastHeartbeatAt = %v, want between %v and %v", session.lastHeartbeatAt, before, after)
	}
	if session.lastHeartbeatAt.Equal(time.Unix(futureTimestamp, 0)) {
		t.Fatalf("lastHeartbeatAt = %v, want server receive time not device timestamp", session.lastHeartbeatAt)
	}
}

func TestAuthHandlerInvalidSecretReturnsError(t *testing.T) {
	gw := newGateway(2 * time.Second)
	ch := &mockChannel{}
	if _, err := onChannel(gw)(context.Background(), ch); err != nil {
		t.Fatalf("onChannel() error = %v", err)
	}

	err := authHandler(gw)(context.Background(), ch, protocol.Auth("dev-001", "wrong-secret"))
	if !errors.Is(err, errInvalidSecret) {
		t.Fatalf("authHandler() error = %v, want %v", err, errInvalidSecret)
	}
}

func TestAuthHandlerDuplicateAuthReturnsError(t *testing.T) {
	gw := newGateway(2 * time.Second)
	ch := &mockChannel{}
	if _, err := onChannel(gw)(context.Background(), ch); err != nil {
		t.Fatalf("onChannel() error = %v", err)
	}
	if err := authHandler(gw)(context.Background(), ch, protocol.Auth("dev-001", demoSecret)); err != nil {
		t.Fatalf("authHandler() first call error = %v", err)
	}

	err := authHandler(gw)(context.Background(), ch, protocol.Auth("dev-001", demoSecret))
	if !errors.Is(err, errDuplicateAuth) {
		t.Fatalf("authHandler() error = %v, want %v", err, errDuplicateAuth)
	}
}

func TestWatchdogClosesStaleChannels(t *testing.T) {
	gw := newGateway(2 * time.Second)
	stale := &mockChannel{}
	fresh := &mockChannel{}
	unauthenticated := &mockChannel{}

	for _, ch := range []*mockChannel{stale, fresh, unauthenticated} {
		if _, err := onChannel(gw)(context.Background(), ch); err != nil {
			t.Fatalf("onChannel() error = %v", err)
		}
	}
	if err := authHandler(gw)(context.Background(), stale, protocol.Auth("dev-stale", demoSecret)); err != nil {
		t.Fatalf("authHandler(stale) error = %v", err)
	}
	if err := authHandler(gw)(context.Background(), fresh, protocol.Auth("dev-fresh", demoSecret)); err != nil {
		t.Fatalf("authHandler(fresh) error = %v", err)
	}

	now := time.Now()
	gw.registry.mu.Lock()
	gw.registry.sessions[stale].lastHeartbeatAt = now.Add(-3 * gw.heartbeatTimeout)
	gw.registry.sessions[fresh].lastHeartbeatAt = now.Add(-gw.heartbeatTimeout / 2)
	gw.registry.mu.Unlock()

	closeStaleSessions(gw, now)

	if !stale.closed {
		t.Fatal("stale channel closed = false, want true")
	}
	if !errors.Is(stale.closeErr, errHeartbeatTimeout) {
		t.Fatalf("stale close err = %v, want %v", stale.closeErr, errHeartbeatTimeout)
	}
	if fresh.closed {
		t.Fatal("fresh channel closed = true, want false")
	}
	if unauthenticated.closed {
		t.Fatal("unauthenticated channel closed = true, want false")
	}
}

func TestNewGatewayServerAcceptsDeviceAuth(t *testing.T) {
	gw := newGateway(2 * time.Second)
	addr, shutdown := startTestGatewayServerEventually(t, gw)
	t.Cleanup(shutdown)

	inbound := make(chan any, 4)
	cli := newTestClient(t, addr, inbound)
	t.Cleanup(func() { cli.Close(nil) })

	if err := dialGatewayClientEventually(t, cli, context.Background()); err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	ch := cli.Channel()
	if ch == nil {
		t.Fatal("Channel() = nil, want active channel")
	}
	if err := ch.Write(protocol.Auth("dev-001", demoSecret)); err != nil {
		t.Fatalf("Write auth failed: %v", err)
	}

	select {
	case got := <-inbound:
		if !reflect.DeepEqual(got, protocol.AuthAck("ok")) {
			t.Fatalf("auth ack = %#v, want %#v", got, protocol.AuthAck("ok"))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for auth ack")
	}

	waitForAuthenticatedSession(t, gw, "dev-001")
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

func TestWaitForGatewayReadyRejectsNonGatewayListener(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = listener.Close() }()

	accepted := make(chan struct{}, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			accepted <- struct{}{}
			_ = conn.Close()
		}
	}()

	gw := newGateway(2 * time.Second)
	err = waitForGatewayReady(listener.Addr().String(), gw, 200*time.Millisecond)
	if err == nil {
		t.Fatal("waitForGatewayReady() error = nil, want failure for non-gateway listener")
	}

	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("waitForGatewayReady() did not connect to listener")
	}
}

func TestDialGatewayClientEventuallyRespectsContextCancellation(t *testing.T) {
	cli := newTestClient(t, "127.0.0.1:65535", make(chan any, 1))
	t.Cleanup(func() { cli.Close(nil) })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	err := dialGatewayClientEventually(t, cli, ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("dialGatewayClientEventually() error = %v, want %v", err, context.Canceled)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("dialGatewayClientEventually() elapsed = %v, want prompt cancellation", elapsed)
	}
}

func waitForAuthenticatedSession(t *testing.T, gw *gateway, deviceID string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if gatewayHasAuthenticatedSession(gw, deviceID) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for authenticated session for %q", deviceID)
}

func startTestGatewayServerEventually(t *testing.T, gw *gateway) (string, func()) {
	t.Helper()

	const maxAttempts = 5

	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		addr := reserveTCPAddr(t)
		srv := newGatewayServer(addr, gw)
		srv.Run()

		if err := waitForGatewayReady(addr, gw, serverReadyTimeout); err == nil {
			return addr, func() { srv.Shutdown(context.Background(), nil) }
		} else {
			lastErr = err
			srv.Shutdown(context.Background(), err)
		}
	}

	t.Fatalf("startTestGatewayServerEventually() error = %v", lastErr)
	return "", nil
}
