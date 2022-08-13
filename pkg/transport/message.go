package transport

import "sync"

// OnMessage defines the function for handling received message
// OnMessage will run in a separate goroutine
type OnMessage func(ctx Context)

type MessageType int

const (
	MessageTypeInbound  MessageType = 0
	MessageTypeOutbound MessageType = 1
)

type Message interface {
	Data() interface{}

	SetData(interface{})

	MessageType() MessageType

	SetTag(key, value interface{})

	GetTag(key interface{}) (interface{}, bool)
}

func NewMessage(msgType MessageType, data interface{}) Message {
	return &message{
		data:    data,
		msgType: msgType,
	}
}

type message struct {
	data    interface{}
	msgType MessageType
	tags    sync.Map
}

func (msg *message) Data() interface{} {
	return msg.data
}

func (msg *message) SetData(data interface{}) {
	msg.data = data
}

func (msg *message) MessageType() MessageType {
	return msg.msgType
}

func (msg *message) SetTag(key, value interface{}) {
	msg.tags.Store(key, value)
}

func (msg *message) GetTag(key interface{}) (interface{}, bool) {
	return msg.tags.Load(key)
}
