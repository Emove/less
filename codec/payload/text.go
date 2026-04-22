package payload

import (
	"errors"
	"github.com/emove/less/codec"
	"github.com/emove/less/internal/engine/framebuf"
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

func (*textPayloadCodec) Marshal(message any) (codec.Frame, error) {
	switch v := message.(type) {
	case string:
		return framebuf.NewFrame([]byte(v)), nil
	case []byte:
		return framebuf.NewFrame(v), nil
	default:
		return nil, ErrMessageNotString
	}
}

func (*textPayloadCodec) Unmarshal(payload codec.Frame) (any, error) {
	return string(payload.Bytes()), nil
}
