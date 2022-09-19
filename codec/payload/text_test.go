package payload

import (
	"bytes"
	"github.com/emove/less/pkg/io"
	"github.com/emove/less/pkg/io/reader"
	"github.com/emove/less/pkg/io/writer"
	"reflect"
	"testing"
)

func TestTextPayloadCodec_Marshal(t *testing.T) {
	type args struct {
		message interface{}
		writer  io.Writer
	}
	buff := &bytes.Buffer{}
	tests := []struct {
		args    args
		want    string
		wantErr bool
	}{
		{
			args:    args{message: "hello world", writer: writer.NewBufferWriterWithBuf(buff)},
			want:    "hello world",
			wantErr: false,
		},
		{
			args:    args{message: 1, writer: writer.NewBufferWriterWithBuf(buff)},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			te := &textPayloadCodec{}
			if err := te.Marshal(tt.args.message, tt.args.writer); (err != nil) && tt.wantErr {
				t.Logf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if buff.String() != tt.want {
				t.Errorf("Marshal() want = %v, got = %v", tt.want, buff.String())
			}
			buff.Reset()
		})
	}
}

func TestTextPayloadCodec_UnMarshal(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		args        args
		wantMessage interface{}
		wantErr     bool
	}{
		{
			args:        args{reader: unMarshalReader("hello world", 11)},
			wantMessage: "hello world",
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			te := &textPayloadCodec{}
			gotMessage, err := te.UnMarshal(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnMarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMessage, tt.wantMessage) {
				t.Errorf("UnMarshal() gotMessage = %v, want %v", gotMessage, tt.wantMessage)
			}
		})
	}
}

func unMarshalReader(msg string, size uint32) io.Reader {
	buff := &bytes.Buffer{}
	buff.WriteString(msg)
	return reader.NewLimitReader(reader.NewBufferReader(buff), size)
}
