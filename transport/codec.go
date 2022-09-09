package transport

import (
	"github.com/emove/less/io"
)

type PacketCodec interface {
	Name() string
	Encode(msg interface{}, writer io.Writer) (err error)
	Decode(reader io.Reader) (msg interface{}, err error)
}
