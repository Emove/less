package packet

import (
	"errors"
	"github.com/emove/less/codec"
	"github.com/emove/less/io"
)

var ErrPayloadExceedsFixedLength = errors.New("payload size exceeds fixed length")

// NewFixedLengthCodec returns a fixed length packet codec
func NewFixedLengthCodec(length uint32) codec.PacketCodec {
	return &fixedLengthCodec{length: length}
}

var _ codec.PacketCodec = (*fixedLengthCodec)(nil)

type fixedLengthCodec struct {
	length uint32
}

func (*fixedLengthCodec) Name() string {
	return "fixed-length-packet-codec"
}

func (c *fixedLengthCodec) Encode(payload []byte, writer io.Writer) error {
	if uint32(len(payload)) > c.length {
		return ErrPayloadExceedsFixedLength
	}

	buf, err := writer.Malloc(int(c.length))
	if err != nil {
		return err
	}

	copy(buf, payload)

	return writer.Flush()
}

func (c *fixedLengthCodec) Decode(reader io.Reader) ([]byte, error) {
	return reader.Next(int(c.length))
}
