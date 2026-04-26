package packet

import (
	"bytes"
	"errors"
	"github.com/emove/less/codec"
)

var ErrMsgSizeGreaterThanMaxLength = errors.New("message package size greater than max length")

type Option func(dc *delimiterCodec)

// DisableAutoAppendDelimiter disables the behavior of automatic appends delimiter when encode
func DisableAutoAppendDelimiter() Option {
	return func(dc *delimiterCodec) {
		dc.autoAppendDelimiter = false
	}
}

// DisableStripDelimiter disable that strip delimiter when decode
func DisableStripDelimiter() Option {
	return func(dc *delimiterCodec) {
		dc.stripDelimiter = false
	}
}

// NewDelimiterCodec returns a packet codec
func NewDelimiterCodec(delimiter string, maxLength uint32, ops ...Option) codec.PacketCodec {
	dc := &delimiterCodec{
		maxLength:           maxLength,
		delimiterLength:     len(delimiter),
		delimiter:           []byte(delimiter),
		autoAppendDelimiter: true,
		stripDelimiter:      true,
	}

	for _, op := range ops {
		op(dc)
	}

	return dc
}

var _ codec.PacketCodec = (*delimiterCodec)(nil)

type delimiterCodec struct {
	maxLength           uint32
	delimiterLength     int
	delimiter           []byte
	autoAppendDelimiter bool
	stripDelimiter      bool
}

func (dc *delimiterCodec) Name() string {
	return "delimiter-packet-codec"
}

func (dc *delimiterCodec) Encode(writer codec.WriterBuffer, payload codec.Frame) error {
	body := payload.Bytes()

	// write payload
	if err := writer.WriteBinary(body); err != nil {
		return err
	}

	// append delimiter
	if dc.autoAppendDelimiter {
		if err := writer.WriteBinary(dc.delimiter); err != nil {
			return err
		}
	}

	return nil
}

func (dc *delimiterCodec) Decode(reader codec.ReaderBuffer) (codec.Frame, error) {

	var peek []byte
	var err error
	length, found := 0, false
	for length = 1; length <= int(dc.maxLength) && !found; length++ {
		peek, err = reader.Peek(length)
		if err != nil {
			return nil, err
		}
		if len(peek) >= dc.delimiterLength && bytes.Equal(peek[length-dc.delimiterLength:], dc.delimiter) {
			found = true
		}
	}
	length--

	if !found {
		return nil, ErrMsgSizeGreaterThanMaxLength
	}

	bodyLength := length
	if dc.stripDelimiter {
		bodyLength -= dc.delimiterLength
	}

	body, err := reader.Slice(bodyLength)
	if err != nil {
		return nil, err
	}

	if dc.stripDelimiter {
		// skip the delimiter bytes
		_ = reader.Skip(dc.delimiterLength)
	}

	return body, nil
}
