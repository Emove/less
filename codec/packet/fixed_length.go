package packet

import (
	"errors"
	"github.com/emove/less/codec"
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

func (c *fixedLengthCodec) Encode(writer codec.WriterBuffer, payload codec.Frame) error {
	body := payload.Bytes()
	if uint32(len(body)) > c.length {
		return ErrPayloadExceedsFixedLength
	}

	buf, err := writer.Malloc(int(c.length))
	if err != nil {
		return err
	}

	copy(buf, body)
	return nil
}

func (c *fixedLengthCodec) Decode(reader codec.ReaderBuffer) (codec.Frame, error) {
	return reader.Slice(int(c.length))
}
