package packet

import (
	"encoding/binary"
	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
	ior "github.com/emove/less/pkg/io/reader"
	iow "github.com/emove/less/pkg/io/writer"
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

func (*variableLengthCodec) Encode(message interface{}, writer io.Writer, payloadCodec codec.PayloadCodec) (err error) {
	header, err := writer.Malloc(binary.MaxVarintLen32)
	if err != nil {
		return
	}

	bufferWriter := iow.WrapBufferWriter(writer)
	defer bufferWriter.Release()

	if err = payloadCodec.Marshal(message, bufferWriter); err != nil {
		return err
	}

	binary.BigEndian.PutUint32(header, uint32(bufferWriter.MallocLength()))

	return writer.Flush()
}

func (*variableLengthCodec) Decode(reader io.Reader, payloadCodec codec.PayloadCodec) (message interface{}, err error) {

	header, err := reader.Next(binary.MaxVarintLen32)
	if err != nil {
		return
	}

	bodyLength := binary.BigEndian.Uint32(header)
	limitReader := ior.NewLimitReader(reader, bodyLength)
	defer limitReader.Release()

	return payloadCodec.UnMarshal(limitReader)
}
