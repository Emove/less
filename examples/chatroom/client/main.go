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
