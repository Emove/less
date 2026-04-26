package chat

import (
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/payload"
)

const (
	TypeSetName = "set_name"
	TypeChat    = "chat"
	TypeSystem  = "system"
)

type Message struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
	Text string `json:"text,omitempty"`
}

func SetName(name string) *Message {
	return &Message{Type: TypeSetName, Name: name}
}

func Chat(name, text string) *Message {
	return &Message{Type: TypeChat, Name: name, Text: text}
}

func System(text string) *Message {
	return &Message{Type: TypeSystem, Text: text}
}

func NewJSONCodec() codec.PayloadCodec {
	return payload.NewJSONCodecWithType(Message{})
}
