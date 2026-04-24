package e2e_test

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

const (
	defaultWaitTimeout = 3 * time.Second
	packetHeaderSize   = binary.MaxVarintLen32
)

type eventCounter struct {
	count int32
}

func (c *eventCounter) inc() {
	atomic.AddInt32(&c.count, 1)
}

func (c *eventCounter) value() int32 {
	return atomic.LoadInt32(&c.count)
}

func (c *eventCounter) waitFor(t *testing.T, want int32) {
	t.Helper()

	waitUntil(t, defaultWaitTimeout, func() bool {
		return c.value() >= want
	}, "event count reached %d, want at least %d", c.value(), want)
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
	deadline := time.Now().Add(defaultWaitTimeout)

	var lastErr error
	for time.Now().Before(deadline) {
		if err := cli.Dial(context.Background()); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(10 * time.Millisecond)
	}
	if lastErr != nil {
		return lastErr
	}
	return context.DeadlineExceeded
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

	var serverOnChannel eventCounter
	var serverClosed eventCounter
	var clientOnChannel eventCounter
	var clientClosed eventCounter

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
		server.WithRouter(func(context.Context, less.Channel, any) (less.Handler, error) {
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
		client.WithRouter(func(context.Context, less.Channel, any) (less.Handler, error) {
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

	clientChannel := cli.Channel()
	if clientChannel == nil {
		t.Fatal("expected client channel to be non-nil")
	}

	var serverChannel less.Channel
	select {
	case serverChannel = <-serverChannelReady:
	case <-time.After(defaultWaitTimeout):
		t.Fatal("expected server OnChannel hook to capture the channel")
	}
	if !serverChannel.IsActive() {
		t.Fatal("expected server channel to be active")
	}

	if err := clientChannel.Write("client-to-server"); err != nil {
		t.Fatalf("client Write failed: %v", err)
	}
	select {
	case got := <-serverReceived:
		if got != "client-to-server" {
			t.Fatalf("server received %q, want %q", got, "client-to-server")
		}
	case <-time.After(defaultWaitTimeout):
		t.Fatal("expected server to receive client message")
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
		t.Fatal("expected client to receive server message")
	}

	cli.Close(errors.New("client requested close"))

	serverClosed.waitFor(t, 1)
	clientClosed.waitFor(t, 1)
	waitUntil(t, defaultWaitTimeout, func() bool {
		return cli.Channel() == nil
	}, "expected client channel to be nil after close")

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

func TestTier1E2E_ConcurrentClients(t *testing.T) {
	const (
		clientCount       = 8
		messagesPerClient = 5
	)

	addr := reserveTCPAddr(t)

	var serverOnChannel eventCounter
	var serverClosed eventCounter
	var serverReceived eventCounter

	srv := newTextServer(
		addr,
		server.WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			serverOnChannel.inc()
			return ctx, nil
		}),
		server.WithOnChannelClosed(func(context.Context, less.Channel, error) {
			serverClosed.inc()
		}),
		server.WithRouter(func(context.Context, less.Channel, any) (less.Handler, error) {
			return func(_ context.Context, ch less.Channel, message any) error {
				serverReceived.inc()
				return ch.Write("ack:" + message.(string))
			}, nil
		}),
	)
	srv.Run()
	t.Cleanup(func() {
		srv.Shutdown(context.Background(), nil)
	})

	errs := make(chan error, clientCount)
	var wg sync.WaitGroup
	wg.Add(clientCount)

	for clientID := 0; clientID < clientCount; clientID++ {
		clientID := clientID
		go func() {
			defer wg.Done()

			acks := make(chan string, messagesPerClient)
			cli := newTextClient(
				addr,
				client.WithRouter(func(context.Context, less.Channel, any) (less.Handler, error) {
					return func(_ context.Context, _ less.Channel, message any) error {
						acks <- message.(string)
						return nil
					}, nil
				}),
			)
			defer cli.Close(nil)

			if err := dialClientWithTimeout(cli); err != nil {
				errs <- fmt.Errorf("client %d dial: %w", clientID, err)
				return
			}

			ch := cli.Channel()
			if ch == nil {
				errs <- fmt.Errorf("client %d channel is nil after dial", clientID)
				return
			}

			for seq := 0; seq < messagesPerClient; seq++ {
				msg := fmt.Sprintf("client=%d seq=%d", clientID, seq)
				if err := ch.Write(msg); err != nil {
					errs <- fmt.Errorf("client %d write seq %d: %w", clientID, seq, err)
					return
				}

				want := "ack:" + msg
				select {
				case got := <-acks:
					if got != want {
						errs <- fmt.Errorf("client %d ack seq %d = %q, want %q", clientID, seq, got, want)
						return
					}
				case <-time.After(defaultWaitTimeout):
					errs <- fmt.Errorf("client %d timed out waiting for ack seq %d", clientID, seq)
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if t.Failed() {
		return
	}

	serverOnChannel.waitFor(t, clientCount)
	if got := serverOnChannel.value(); got != clientCount {
		t.Fatalf("server OnChannel count = %d, want %d", got, clientCount)
	}

	wantReceived := int32(clientCount * messagesPerClient)
	if got := serverReceived.value(); got != wantReceived {
		t.Fatalf("server received count = %d, want %d", got, wantReceived)
	}

	serverClosed.waitFor(t, clientCount)
	if got := serverClosed.value(); got != clientCount {
		t.Fatalf("server OnChannelClosed count = %d, want %d", got, clientCount)
	}
}
