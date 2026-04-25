package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/examples/device-gateway/protocol"
)

const (
	defaultAddr              = "127.0.0.1:9000"
	defaultDeviceID          = "dev-001"
	defaultSecret            = "demo-secret"
	defaultHeartbeatInterval = time.Second
	authAckTimeout           = 3 * time.Second
)

func main() {
	addr := flag.String("addr", defaultAddr, "gateway server address")
	deviceID := flag.String("device-id", defaultDeviceID, "device identifier")
	secret := flag.String("secret", defaultSecret, "device shared secret")
	flag.Parse()

	if *deviceID == "" {
		fmt.Fprintln(os.Stderr, "device-id cannot be empty")
		os.Exit(1)
	}
	if *secret == "" {
		fmt.Fprintln(os.Stderr, "secret cannot be empty")
		os.Exit(1)
	}

	runtimeCtx, cancelRuntime := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelRuntime()

	authAcks := make(chan *protocol.AuthAckMessage, 1)
	cli := newDeviceClient(*addr, authAcks, cancelRuntime)
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

	if err := ch.Write(protocol.Auth(*deviceID, *secret)); err != nil {
		fmt.Fprintf(os.Stderr, "auth failed: %v\n", err)
		os.Exit(1)
	}

	authAck, err := waitForAuthAck(runtimeCtx, authAcks, authAckTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "auth ack failed: %v\n", err)
		os.Exit(1)
	}
	if authAck.Status != "ok" {
		fmt.Fprintf(os.Stderr, "auth rejected with status %q\n", authAck.Status)
		os.Exit(1)
	}

	if err := ch.Write(protocol.Telemetry(map[string]string{
		"battery": "98",
		"temp":    "23.4",
	})); err != nil {
		fmt.Fprintf(os.Stderr, "telemetry failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("telemetry sent")

	go runHeartbeats(runtimeCtx, ch, defaultHeartbeatInterval, cancelRuntime)

	<-runtimeCtx.Done()
	cli.Close(runtimeCtx.Err())
}

func newDeviceClient(addr string, authAcks chan<- *protocol.AuthAckMessage, cancel func()) *client.Client {
	return client.NewClient(
		"tcp",
		addr,
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(protocol.NewCodec()),
		client.WithOnChannelClosed(func(_ context.Context, _ less.Channel, err error) {
			if err != nil {
				fmt.Fprintf(os.Stderr, "channel closed: %v\n", err)
			} else {
				fmt.Fprintln(os.Stderr, "channel closed")
			}
			cancel()
		}),
		client.WithRouter(deviceRouter(authAcks)),
	)
}

func deviceRouter(authAcks chan<- *protocol.AuthAckMessage) less.Router {
	return func(_ context.Context, _ less.Channel, _ interface{}) (less.Handler, error) {
		return func(_ context.Context, ch less.Channel, msg interface{}) error {
			switch message := msg.(type) {
			case *protocol.AuthAckMessage:
				fmt.Printf("auth_ack status=%s\n", message.Status)
				select {
				case authAcks <- message:
				default:
				}
				return nil
			case *protocol.CommandMessage:
				fmt.Printf("command request_id=%d name=%s\n", message.RequestID, message.Name)
				if err := ch.Write(protocol.CommandAck(message.RequestID, "ok")); err != nil {
					return fmt.Errorf("reply command ack: %w", err)
				}
				fmt.Printf("command_ack request_id=%d status=ok\n", message.RequestID)
				return nil
			default:
				fmt.Printf("inbound type=%T\n", msg)
				return nil
			}
		}, nil
	}
}

func waitForAuthAck(ctx context.Context, authAcks <-chan *protocol.AuthAckMessage, timeout time.Duration) (*protocol.AuthAckMessage, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case authAck := <-authAcks:
		return authAck, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("timed out after %v", timeout)
	}
}

func runHeartbeats(ctx context.Context, ch less.Channel, interval time.Duration, cancel func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := ch.Write(protocol.Heartbeat(time.Now().Unix())); err != nil {
				fmt.Fprintf(os.Stderr, "heartbeat failed: %v\n", err)
				cancel()
				return
			}
			fmt.Println("heartbeat sent")
		}
	}
}
