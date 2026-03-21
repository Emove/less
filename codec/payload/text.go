package payload

import (
	"errors"
	"github.com/emove/less/codec"
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

func (*textPayloadCodec) Marshal(message any) ([]byte, error) {
	switch v := message.(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return nil, ErrMessageNotString
	}
}

func (*textPayloadCodec) Unmarshal(payload []byte) (any, error) {
	return string(payload), nil
}
