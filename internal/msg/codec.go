package msg

import (
	"encoding/binary"

	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
)

const (
	MAGICLength = 4
	TypeLength  = 2
)

func NewLessMsgPayloadCodec(c codec.PayloadCodec) codec.PayloadCodec {
	return &LessMessagePayloadCodec{
		c: c,
	}
}

type LessMessagePayloadCodec struct {
	c codec.PayloadCodec
}

func (*LessMessagePayloadCodec) Name() string {
	return "less-message-payload-codec"
}

func (lc *LessMessagePayloadCodec) Marshal(message interface{}, writer io.Writer) (err error) {
	msg, ok := message.(*LessMessage)
	if !ok {
		return lc.c.Marshal(message, writer)
	}

	// write magic
	magic, err := writer.Malloc(MAGICLength)
	if err != nil {
		return
	}
	binary.BigEndian.PutUint32(magic, msg.Magic)

	// write msg type
	msgType, err := writer.Malloc(TypeLength)
	if err != nil {
		return
	}
	binary.BigEndian.PutUint16(msgType, msg.MsgType)

	// write body
	_, err = writer.Write(msg.Body)

	return
}

func (lc *LessMessagePayloadCodec) UnMarshal(reader io.Reader) (message interface{}, err error) {
	if reader.Length() < MAGICLength+TypeLength {
		return lc.c.UnMarshal(reader)
	}
	// check magic
	magic, err := reader.Peek(MAGICLength)
	if err != nil {
		return
	}
	if binary.BigEndian.Uint32(magic) != MAGIC {
		return lc.c.UnMarshal(reader)
	}
	_ = reader.Skip(MAGICLength)

	// read msg type
	msgType, err := reader.Next(TypeLength)
	if err != nil {
		return nil, err
	}
	msg := &LessMessage{
		Magic:   MAGIC,
		MsgType: binary.BigEndian.Uint16(msgType),
	}
	msg.Body, err = reader.Next(reader.Length() - MAGICLength - TypeLength)
	return msg, err

}
