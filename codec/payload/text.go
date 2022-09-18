package payload

import (
	"errors"
	"github.com/emove/less/pkg/io"
)

var ErrMessageNotString = errors.New("message can not convert to string")

type TextPayloadCodec struct {
}

func (*TextPayloadCodec) Name() string {
	return "text-payload-codec"
}

func (*TextPayloadCodec) Marshal(message interface{}, writer io.Writer) (err error) {
	content, ok := message.(string)
	if !ok {
		return ErrMessageNotString
	}

	_, err = writer.Write([]byte(content))
	return
}

func (*TextPayloadCodec) UnMarshal(reader io.Reader) (message interface{}, err error) {
	content, err := reader.Next(reader.Length())
	if err != nil {
		return
	}

	return string(content), nil
}
