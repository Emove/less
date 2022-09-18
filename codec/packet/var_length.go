package packet

import (
	"encoding/binary"
	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
	ior "github.com/emove/less/pkg/io/reader"
	iow "github.com/emove/less/pkg/io/writer"
)

type VariableLengthCodec struct{}

func (*VariableLengthCodec) Name() string {
	return "variable-length-packet-codec"
}

func (*VariableLengthCodec) Encode(message interface{}, writer io.Writer, payloadCodec codec.PayloadCodec) (err error) {
	header := writer.Malloc(binary.MaxVarintLen32)

	bufferWriter := iow.NewBufferWriter(writer)
	defer bufferWriter.Release()

	if err = payloadCodec.Marshal(message, bufferWriter); err != nil {
		return err
	}

	binary.BigEndian.PutUint32(header, uint32(bufferWriter.MallocLength()))

	return writer.Flush()
}

func (*VariableLengthCodec) Decode(reader io.Reader, payloadCodec codec.PayloadCodec) (message interface{}, err error) {

	header, err := reader.Next(binary.MaxVarintLen32)
	if err != nil {
		return
	}

	bodyLength := binary.BigEndian.Uint32(header)
	limitReader := ior.NewLimitReader(reader, bodyLength)
	defer limitReader.Release()

	return payloadCodec.UnMarshal(limitReader)
}
