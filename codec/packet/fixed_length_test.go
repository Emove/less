package packet

import (
	"bytes"
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/payload"
	less_io "github.com/emove/less/pkg/io"
	ior "github.com/emove/less/pkg/io/reader"
	"github.com/emove/less/pkg/io/writer"
	"io"
	"reflect"
	"testing"
)

type testReader struct {
	buffer *bytes.Buffer
}

func newTestReader(content []byte) io.Reader {
	return &testReader{
		buffer: bytes.NewBuffer(content),
	}
}

func (r *testReader) Read(buf []byte) (n int, err error) {
	return r.buffer.Read(buf)
}

func Test_fixedLengthCodec_Decode(t *testing.T) {
	type fields struct {
		length uint32
	}
	type args struct {
		reader       less_io.Reader
		payloadCodec codec.PayloadCodec
	}
	tests := []struct {
		name        string
		times       int
		fields      fields
		args        args
		wantMessage []string
		wantErr     bool
	}{
		{
			name:        "first",
			times:       1,
			fields:      fields{length: 8},
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("12345678"))), payloadCodec: payload.NewTextCodec()},
			wantMessage: []string{"12345678"},
			wantErr:     false,
		},
		{
			name:        "second",
			times:       2,
			fields:      fields{length: 8},
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567887654321"))), payloadCodec: payload.NewTextCodec()},
			wantMessage: []string{"12345678", "87654321"},
			wantErr:     false,
		},
		{
			name:        "third",
			times:       1,
			fields:      fields{length: 8},
			args:        args{reader: ior.NewLimitReader(ior.NewBufferReader(newTestReader([]byte("1234567"))), 7), payloadCodec: payload.NewTextCodec()},
			wantMessage: nil,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		for i := 0; i < tt.times; i++ {
			t.Run(tt.name, func(t *testing.T) {
				c := &fixedLengthCodec{
					length: tt.fields.length,
				}
				gotMessage, err := c.Decode(tt.args.reader, tt.args.payloadCodec)
				if (err != nil) != tt.wantErr {
					t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if tt.wantMessage != nil && !reflect.DeepEqual(gotMessage, tt.wantMessage[i]) {
					t.Errorf("Decode() gotMessage = %v, want %v", gotMessage, tt.wantMessage)
				}
			})
		}
	}
}

func Test_fixedLengthCodec_Encode(t *testing.T) {
	type fields struct {
		length uint32
	}
	type args struct {
		writer       less_io.Writer
		payloadCodec codec.PayloadCodec
	}
	buf := &bytes.Buffer{}
	tests := []struct {
		name    string
		fields  fields
		msgs    []string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name:    "first",
			fields:  fields{length: 8},
			msgs:    []string{"12345678"},
			args:    args{writer: writer.NewBufferWriter(buf), payloadCodec: payload.NewTextCodec()},
			want:    "12345678",
			wantErr: false,
		},
		{
			name:    "second",
			fields:  fields{length: 7},
			msgs:    []string{"12345678"},
			args:    args{writer: writer.NewBufferWriter(buf), payloadCodec: payload.NewTextCodec()},
			want:    "",
			wantErr: true,
		},
		{
			name:    "third",
			fields:  fields{length: 8},
			msgs:    []string{"12345678", "87654321"},
			args:    args{writer: writer.NewBufferWriter(buf), payloadCodec: payload.NewTextCodec()},
			want:    "1234567887654321",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		for _, msg := range tt.msgs {
			t.Run(tt.name, func(t *testing.T) {
				c := &fixedLengthCodec{
					length: tt.fields.length,
				}
				if err := c.Encode(msg, tt.args.writer, tt.args.payloadCodec); (err != nil) != tt.wantErr {
					t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
		if buf.String() != tt.want {
			t.Errorf("Encode() error, want = %s, got %s", tt.want, buf.String())
		}
		buf.Reset()
	}
}
