package codec

import (
	"github.com/emove/less/pkg/io"
)

type PacketCodec interface {
	Name() string
	Encode(message interface{}, writer io.Writer, payloadCodec PayloadCodec) (err error)
	Decode(reader io.Reader, payloadCodec PayloadCodec) (message interface{}, err error)
}

type PayloadCodec interface {
	Name() string
	Marshal(message interface{}, writer io.Writer) (err error)
	UnMarshal(reader io.Reader) (message interface{}, err error)
}
