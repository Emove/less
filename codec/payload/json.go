package payload

import (
	"encoding/json"
	"github.com/emove/less/codec"
	"github.com/emove/less/internal/engine/framebuf"
	"reflect"
)

// NewJSONCodecWithType uses reflect to new an instance of message type when unmarshal, which got a better performance than using map
func NewJSONCodecWithType(msg interface{}) codec.PayloadCodec {
	return &jsonPayloadCodec{
		msgType: parseType(msg),
	}
}

// NewJSONCodec returns a json payload codec that using map as receiver when json unmarshal
func NewJSONCodec() codec.PayloadCodec {
	return &jsonPayloadCodec{}
}

var _ codec.PayloadCodec = (*jsonPayloadCodec)(nil)

type jsonPayloadCodec struct {
	msgType reflect.Type
}

func (*jsonPayloadCodec) Name() string {
	return "json-payload-codec"
}

func (*jsonPayloadCodec) Marshal(message any) (codec.Frame, error) {
	b, err := json.Marshal(message)
	if err != nil {
		return nil, err
	}
	return framebuf.NewFrame(b), nil
}

func (jpc *jsonPayloadCodec) Unmarshal(payload codec.Frame) (any, error) {
	body := payload.Bytes()
	if jpc.msgType != nil {
		message := reflect.New(jpc.msgType).Interface()
		err := json.Unmarshal(body, message)
		return message, err
	}

	// unmarshal to map
	message := make(map[string]interface{})
	err := json.Unmarshal(body, &message)
	return message, err
}

func parseType(msg interface{}) reflect.Type {
	if msg == nil {
		panic("msg type can not be nil")
	}

	if reflect.TypeOf(msg).Kind() == reflect.Ptr {
		return reflect.Indirect(reflect.ValueOf(msg)).Type()
	}
	return reflect.TypeOf(msg)
}
