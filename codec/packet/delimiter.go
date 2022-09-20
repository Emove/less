package packet

import (
	"errors"
	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
	ior "github.com/emove/less/pkg/io/reader"
)

var ErrMsgSizeGreaterThanMaxLength = errors.New("message package size greater than max length")

type Option func(dc *delimiterCodec)

// DisableAutoAppendDelimiter appends delimiter when encode, the default value is true
func DisableAutoAppendDelimiter(append bool) Option {
	return func(dc *delimiterCodec) {
		dc.autoAppendDelimiter = append
	}
}

// DisableStripDelimiter strip delimiter when decode, the default value is true
func DisableStripDelimiter(strip bool) Option {
	return func(dc *delimiterCodec) {
		dc.stripDelimiter = strip
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

func (dc *delimiterCodec) Encode(message interface{}, writer io.Writer, payloadCodec codec.PayloadCodec) (err error) {

	// marshal message and write to writer
	if err = payloadCodec.Marshal(message, writer); err != nil {
		return
	}

	// append delimiter
	if dc.autoAppendDelimiter {
		if _, err = writer.Write(dc.delimiter); err != nil {
			return
		}
	}

	return writer.Flush()
}

func (dc *delimiterCodec) Decode(reader io.Reader, payloadCodec codec.PayloadCodec) (message interface{}, err error) {

	var peek []byte
	length, found := 0, false
	for length = dc.delimiterLength; length <= int(dc.maxLength) && !found; length += dc.delimiterLength {
		peek, err = reader.Peek(length)
		if err != nil {
			return nil, err
		}
		if string(peek[length-dc.delimiterLength:]) == string(dc.delimiter) {
			found = true
		}
	}
	length -= dc.delimiterLength

	if !found {
		return nil, ErrMsgSizeGreaterThanMaxLength
	}

	if dc.stripDelimiter {
		// strip delimiter length
		length -= dc.delimiterLength
	}

	limiterReader := ior.NewLimitReader(reader, uint32(length))
	defer func() {
		limiterReader.Release()
		if dc.stripDelimiter {
			// release the delimiter buff manually
			_ = reader.Skip(dc.delimiterLength)
		}
	}()

	return payloadCodec.UnMarshal(limiterReader)
}
