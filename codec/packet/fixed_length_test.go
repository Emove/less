package packet

import (
	"bytes"
	less_io "github.com/emove/less/io"
	ior "github.com/emove/less/io/reader"
	"github.com/emove/less/io/writer"
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
		reader less_io.Reader
	}
	tests := []struct {
		name        string
		times       int
		fields      fields
		args        args
		wantPayload [][]byte
		wantErr     bool
	}{
		{
			name:        "first",
			times:       1,
			fields:      fields{length: 8},
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("12345678")))},
			wantPayload: [][]byte{[]byte("12345678")},
			wantErr:     false,
		},
		{
			name:        "second",
			times:       2,
			fields:      fields{length: 8},
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567887654321")))},
			wantPayload: [][]byte{[]byte("12345678"), []byte("87654321")},
			wantErr:     false,
		},
		{
			name:        "third",
			times:       1,
			fields:      fields{length: 8},
			args:        args{reader: ior.NewLimitReader(ior.NewBufferReader(newTestReader([]byte("1234567"))), 7)},
			wantPayload: nil,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		for i := 0; i < tt.times; i++ {
			t.Run(tt.name, func(t *testing.T) {
				c := &fixedLengthCodec{
					length: tt.fields.length,
				}
				gotPayload, err := c.Decode(tt.args.reader)
				if (err != nil) != tt.wantErr {
					t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if tt.wantPayload != nil && !reflect.DeepEqual(gotPayload, tt.wantPayload[i]) {
					t.Errorf("Decode() gotPayload = %v, want %v", gotPayload, tt.wantPayload[i])
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
		writer less_io.Writer
	}
	buf := &bytes.Buffer{}
	tests := []struct {
		name    string
		fields  fields
		msgs    []string
		args    args
		want    string
		wantErr bool
	}{
		{
			name:    "first",
			fields:  fields{length: 8},
			msgs:    []string{"12345678"},
			args:    args{writer: writer.NewBufferWriter(buf)},
			want:    "12345678",
			wantErr: false,
		},
		{
			name:    "second",
			fields:  fields{length: 7},
			msgs:    []string{"12345678"},
			args:    args{writer: writer.NewBufferWriter(buf)},
			want:    "",
			wantErr: true,
		},
		{
			name:    "third",
			fields:  fields{length: 8},
			msgs:    []string{"12345678", "87654321"},
			args:    args{writer: writer.NewBufferWriter(buf)},
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
				if err := c.Encode([]byte(msg), tt.args.writer); (err != nil) != tt.wantErr {
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
