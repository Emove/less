package payload

import (
	"errors"
	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
)

var ErrMessageNotString = errors.New("message can not convert to string")

func NewTextCodec() codec.PayloadCodec {
	return &textPayloadCodec{}
}

type textPayloadCodec struct {
}

func (*textPayloadCodec) Name() string {
	return "text-payload-codec"
}

func (*textPayloadCodec) Marshal(message interface{}, writer io.Writer) (err error) {
	content, ok := message.(string)
	if !ok {
		return ErrMessageNotString
	}

	_, err = writer.Write([]byte(content))
	return
}

func (*textPayloadCodec) UnMarshal(reader io.Reader) (message interface{}, err error) {
	content, err := reader.Next(reader.Length())
	if err != nil {
		return
	}

	return string(content), nil
}
