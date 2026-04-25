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
