package packet

import (
	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
	ior "github.com/emove/less/pkg/io/reader"
	iow "github.com/emove/less/pkg/io/writer"
)

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

func (c *fixedLengthCodec) Encode(message interface{}, writer io.Writer, payloadCodec codec.PayloadCodec) (err error) {

	// allocate a fixed length buffer
	buf, err := writer.Malloc(int(c.length))
	if err != nil {
		return err
	}
	bufWriter := iow.NewBufferWriterWithBuff(buf)
	if err = payloadCodec.Marshal(message, bufWriter); err != nil {
		return err
	}

	return writer.Flush()
}

func (c *fixedLengthCodec) Decode(reader io.Reader, payloadCodec codec.PayloadCodec) (message interface{}, err error) {
	limitReader := ior.NewLimitReader(reader, c.length)
	defer limitReader.Release()

	return payloadCodec.UnMarshal(limitReader)
}
