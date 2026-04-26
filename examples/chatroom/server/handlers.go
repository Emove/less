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
