package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/examples/device-gateway/protocol"
	"github.com/emove/less/server"
)

const (
	defaultHeartbeatTimeout = 5 * time.Second
	defaultWatchdogInterval = 500 * time.Millisecond
	serverReadyTimeout      = 3 * time.Second
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9000", "gateway listen address")
	flag.Parse()

	if err := checkListenAddressAvailable(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "device gateway failed to listen on %s: %v\n", *addr, err)
		os.Exit(1)
	}

	gw := newGateway(defaultHeartbeatTimeout)
	srv := newGatewayServer(*addr, gw)
	srv.Run()
	if err := waitForGatewayReady(*addr, gw, serverReadyTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "device gateway failed to listen on %s: %v\n", *addr, err)
		srv.Shutdown(context.Background(), err)
		os.Exit(1)
	}

	watchdogCtx, cancelWatchdog := context.WithCancel(context.Background())
	startWatchdog(watchdogCtx, gw, defaultWatchdogInterval)

	fmt.Printf("device gateway listening on %s\n", *addr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	cancelWatchdog()
	srv.Shutdown(context.Background(), nil)
}

func newGatewayServer(addr string, gw *gateway) *server.Server {
	return server.NewServer(
		addr,
		server.WithPacketCodec(packet.NewVariableLengthCodec()),
		server.WithPayloadCodec(protocol.NewCodec()),
		server.WithOnChannel(onChannel(gw)),
		server.WithOnChannelClosed(onChannelClosed(gw)),
		server.WithRouter(newRouter(gw)),
	)
}

func reserveTCPAddr(t interface {
	Fatalf(string, ...any)
	Helper()
}) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	defer func() { _ = listener.Close() }()

	return listener.Addr().String()
}

func checkListenAddressAvailable(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	return listener.Close()
}

func waitForGatewayReady(addr string, gw *gateway, timeout time.Duration) error {
	if gw == nil {
		return fmt.Errorf("gateway readiness probe requires a gateway instance")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	deviceID := fmt.Sprintf("__gateway_probe__%d", time.Now().UnixNano())
	inbound := make(chan any, 1)
	cli := newGatewayClient(addr, inbound)
	defer cli.Close(nil)

	if err := dialGatewayClientEventuallyContext(cli, ctx); err != nil {
		return err
	}

	ch := cli.Channel()
	if ch == nil {
		return fmt.Errorf("gateway readiness probe connected without an active channel")
	}
	if err := ch.Write(protocol.Auth(deviceID, demoSecret)); err != nil {
		return err
	}

	authAckReceived := false
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		if authAckReceived && gatewayHasAuthenticatedSession(gw, deviceID) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg := <-inbound:
			authAck, ok := msg.(*protocol.AuthAckMessage)
			if !ok {
				continue
			}
			if authAck.Status != "ok" {
				return fmt.Errorf("gateway readiness probe received auth status %q", authAck.Status)
			}
			authAckReceived = true
		case <-ticker.C:
		}
	}
}

func newTestClient(t interface {
	Fatalf(string, ...any)
	Helper()
}, addr string, inbound chan<- any) *client.Client {
	t.Helper()

	return newGatewayClient(addr, inbound)
}

func newGatewayClient(addr string, inbound chan<- any) *client.Client {
	return client.NewClient(
		"tcp",
		addr,
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(protocol.NewCodec()),
		client.WithRouter(func(context.Context, less.Channel, interface{}) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, msg interface{}) error {
				select {
				case inbound <- msg:
				default:
				}
				return nil
			}, nil
		}),
	)
}

func dialGatewayClientEventually(t interface {
	Fatalf(string, ...any)
	Helper()
}, cli *client.Client, ctx context.Context) error {
	t.Helper()

	return dialGatewayClientEventuallyContext(cli, ctx)
}

func dialGatewayClientEventuallyContext(cli *client.Client, ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	deadline := time.Now().Add(3 * time.Second)
	if deadlineFromCtx, ok := ctx.Deadline(); ok && deadlineFromCtx.Before(deadline) {
		deadline = deadlineFromCtx
	}

	var lastErr error
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := cli.Dial(ctx); err == nil {
			return nil
		} else {
			if err := ctx.Err(); err != nil {
				return err
			}
			lastErr = err
		}

		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return context.DeadlineExceeded
		}

		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func gatewayHasAuthenticatedSession(gw *gateway, deviceID string) bool {
	gw.registry.mu.RLock()
	defer gw.registry.mu.RUnlock()

	for _, sess := range gw.registry.sessions {
		if sess.deviceID == deviceID && sess.authenticated {
			return true
		}
	}

	return false
}
