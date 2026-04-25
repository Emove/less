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

	"github.com/emove/less/codec/packet"
	"github.com/emove/less/examples/chatroom/chat"
	"github.com/emove/less/server"
)

const serverReadyTimeout = 3 * time.Second

func main() {
	addr := flag.String("addr", "127.0.0.1:8888", "chatroom listen address")
	flag.Parse()

	if err := checkListenAddressAvailable(*addr); err != nil {
		fmt.Fprintf(os.Stderr, "chatroom server failed to listen on %s: %v\n", *addr, err)
		os.Exit(1)
	}

	h := newHub()
	srv := newChatServer(*addr, h)
	srv.Run()
	if err := waitForServerReady(*addr, serverReadyTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "chatroom server failed to listen on %s: %v\n", *addr, err)
		srv.Shutdown(context.Background(), err)
		os.Exit(1)
	}
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

func checkListenAddressAvailable(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return listener.Close()
}

func waitForServerReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return context.DeadlineExceeded
		}
		time.Sleep(10 * time.Millisecond)
	}
}
