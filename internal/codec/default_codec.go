package codec

import (
	"bytes"
	"encoding/gob"
	"less/pkg/transport"
	"reflect"
	"unsafe"
)

type DefaultCodec struct {
}

func (c DefaultCodec) Encode(res interface{}, writer transport.Writer) error {
}

func (c DefaultCodec) Decode(reader transport.Reader) (req interface{}, err error) {

}
