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
