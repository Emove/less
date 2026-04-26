package packet

import (
	"encoding/binary"
	"github.com/emove/less/codec"
)

// NewVariableLengthCodec returns a variable length packet codec
func NewVariableLengthCodec() codec.PacketCodec {
	return &variableLengthCodec{}
}

type variableLengthCodec struct{}

var _ codec.PacketCodec = (*variableLengthCodec)(nil)

func (*variableLengthCodec) Name() string {
	return "variable-length-packet-codec"
}

func (*variableLengthCodec) Encode(writer codec.WriterBuffer, payload codec.Frame) error {
	body := payload.Bytes()
	header, err := writer.Malloc(binary.MaxVarintLen32)
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint32(header, uint32(len(body)))

	if err = writer.WriteFrame(payload); err != nil {
		return err
	}

	return nil
}

func (*variableLengthCodec) Decode(reader codec.ReaderBuffer) (codec.Frame, error) {
	header, err := reader.Next(binary.MaxVarintLen32)
	if err != nil {
		return nil, err
	}

	bodyLength := binary.BigEndian.Uint32(header)
	return reader.Slice(int(bodyLength))
}
