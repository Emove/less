package packet

import (
	"encoding/binary"
	"github.com/emove/less/codec"
	"github.com/emove/less/io"
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

func (*variableLengthCodec) Encode(payload []byte, writer io.Writer) error {
	header, err := writer.Malloc(binary.MaxVarintLen32)
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint32(header, uint32(len(payload)))

	if _, err = writer.Write(payload); err != nil {
		return err
	}

	return writer.Flush()
}

func (*variableLengthCodec) Decode(reader io.Reader) ([]byte, error) {
	header, err := reader.Next(binary.MaxVarintLen32)
	if err != nil {
		return nil, err
	}

	bodyLength := binary.BigEndian.Uint32(header)
	body, err := reader.Next(int(bodyLength))
	if err != nil {
		return nil, err
	}

	return body, nil
}
