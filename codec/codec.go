package codec

import (
	"github.com/emove/less/io"
)

type PacketCodec interface {
	Name() string
	Encode(payload []byte, writer io.Writer) error
	Decode(reader io.Reader) (payload []byte, err error)
}

type PayloadCodec interface {
	Name() string
	Marshal(message any) ([]byte, error)
	Unmarshal(payload []byte) (any, error)
}
