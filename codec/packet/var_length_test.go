package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/pkg/io"
	reader2 "github.com/emove/less/pkg/io/reader"
	"github.com/emove/less/pkg/io/writer"
	"reflect"
	"testing"
)

func TestVariableLengthCodec_Decode(t *testing.T) {
	type args struct {
		reader       io.Reader
		payloadCodec codec.PayloadCodec
	}
	tests := []struct {
		args        args
		wantMessage interface{}
		wantErr     bool
	}{
		{
			args:        args{reader: reader([]byte("hello world")), payloadCodec: &payload.TextPayloadCodec{}},
			wantMessage: "hello world",
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			va := &VariableLengthCodec{}
			gotMessage, err := va.Decode(tt.args.reader, tt.args.payloadCodec)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMessage, tt.wantMessage) {
				t.Errorf("Decode() gotMessage = %v, want %v", gotMessage, tt.wantMessage)
			}
		})
	}
}

func TestVariableLengthCodec_Encode(t *testing.T) {
	type args struct {
		message      interface{}
		writer       io.Writer
		payloadCodec codec.PayloadCodec
	}
	buff := &bytes.Buffer{}
	tests := []struct {
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			args:    args{message: "hello world", writer: writer.NewBufferWriterWithBuf(buff), payloadCodec: &payload.TextPayloadCodec{}},
			want:    "hello world",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			va := &VariableLengthCodec{}
			if err := va.Encode(tt.args.message, tt.args.writer, tt.args.payloadCodec); (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
			got := buff.Bytes()[binary.MaxVarintLen32:]
			if string(got) != tt.want {
				t.Errorf("Encode() want = %v, got = %v", tt.want, string(got))
			}
			buff.Reset()
		})
	}
}

func reader(msg []byte) io.Reader {
	buff := &bytes.Buffer{}
	header := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(header, uint32(len(msg)))
	buff.Write(header)
	buff.Write(msg)
	return reader2.NewBufferReader(buff)
}
