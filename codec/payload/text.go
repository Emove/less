package payload

import (
	"errors"
	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
)

var ErrMessageNotString = errors.New("message can not convert to string")

// NewTextCodec returns a text payload codec
func NewTextCodec() codec.PayloadCodec {
	return &textPayloadCodec{}
}

var _ codec.PayloadCodec = (*textPayloadCodec)(nil)

type textPayloadCodec struct {
}

func (*textPayloadCodec) Name() string {
	return "text-payload-codec"
}

func (*textPayloadCodec) Marshal(message interface{}, writer io.Writer) (err error) {
	switch message.(type) {
	case string:
		_, err = writer.Write([]byte(message.(string)))
	case []byte:
		_, err = writer.Write(message.([]byte))
	default:
		return ErrMessageNotString
	}
	return
}

func (*textPayloadCodec) UnMarshal(reader io.Reader) (message interface{}, err error) {
	content, err := reader.Next(reader.Length())
	if err != nil {
		return
	}

	return string(content), nil
}
