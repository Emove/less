package payload

import (
	"encoding/json"
	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
	"reflect"
)

// NewJSONCodecWithType uses reflect to new an instance of message type when unmarshal, which got a better performance than using map
func NewJSONCodecWithType(msg interface{}) codec.PayloadCodec {
	return &jsonPayloadCodec{
		msgType: parseType(msg),
	}
}

// NewJSONCodec uses map when json unmarshal
func NewJSONCodec() codec.PayloadCodec {
	return &jsonPayloadCodec{}
}

type jsonPayloadCodec struct {
	msgType reflect.Type
}

func (*jsonPayloadCodec) Name() string {
	return "json-payload-codec"
}

func (*jsonPayloadCodec) Marshal(message interface{}, writer io.Writer) (err error) {
	marshal, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = writer.Write(marshal)
	return err
}

func (jpc *jsonPayloadCodec) UnMarshal(reader io.Reader) (message interface{}, err error) {
	msg, err := reader.Next(reader.Length())
	if err != nil {
		return nil, err
	}

	if jpc.msgType != nil {
		message = reflect.New(jpc.msgType).Interface()
		err = json.Unmarshal(msg, message)
		return
	}

	// unmarshal to map
	message = make(map[string]interface{})
	err = json.Unmarshal(msg, &message)
	return
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
