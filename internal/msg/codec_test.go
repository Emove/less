package msg

import (
	"bytes"
	"github.com/emove/less/pkg/io/reader"
	"github.com/emove/less/pkg/io/writer"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLessMessagePayloadCodec(t *testing.T) {
	codec := LessMessagePayloadCodec{}
	buf := &bytes.Buffer{}
	msg := &LessMessage{Magic: MAGIC, MsgType: 1, Body: []byte("ping")}

	bufferWriter := writer.NewBufferWriter(buf)
	err := codec.Marshal(msg, bufferWriter)
	if err != nil {
		t.Fatal(err)
	}

	_ = bufferWriter.Flush()

	r := reader.NewLimitReader(reader.NewBufferReader(buf), uint32(buf.Len()))

	marshal, err := codec.UnMarshal(r)
	if err != nil {
		t.Fatal(err)
	}

	m := marshal.(*LessMessage)
	assert.Equal(t, m, msg)
}
