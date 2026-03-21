package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/emove/less/io"
	reader2 "github.com/emove/less/io/reader"
	"github.com/emove/less/io/writer"
	"reflect"
	"testing"
)

func TestVariableLengthCodec_Decode(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		args        args
		wantPayload []byte
		wantErr     bool
	}{
		{
			args:        args{reader: reader([]byte("hello world"))},
			wantPayload: []byte("hello world"),
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			va := &variableLengthCodec{}
			gotPayload, err := va.Decode(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotPayload, tt.wantPayload) {
				t.Errorf("Decode() gotPayload = %v, want %v", gotPayload, tt.wantPayload)
			}
		})
	}
}

func TestVariableLengthCodec_Encode(t *testing.T) {
	type args struct {
		payload []byte
		writer  io.Writer
	}
	buff := &bytes.Buffer{}
	tests := []struct {
		args    args
		want    string
		wantErr bool
	}{
		{
			args:    args{payload: []byte("hello world"), writer: writer.NewBufferWriter(buff)},
			want:    "hello world",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			va := &variableLengthCodec{}
			if err := va.Encode(tt.args.payload, tt.args.writer); (err != nil) != tt.wantErr {
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
