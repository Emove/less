package packet

import (
	"bytes"
	"github.com/emove/less/internal/engine/framebuf"
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

func TestFixedLengthCodec_DecodeReturnsFrame(t *testing.T) {
	c := &fixedLengthCodec{length: 8}

	gotPayload, err := c.Decode(framebuf.NewReader(newTestReader([]byte("12345678"))))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	defer gotPayload.Release()

	if !reflect.DeepEqual(gotPayload.Bytes(), []byte("12345678")) {
		t.Fatalf("Decode() frame bytes = %v, want %v", gotPayload.Bytes(), []byte("12345678"))
	}
}

func Test_fixedLengthCodec_Encode(t *testing.T) {
	type fields struct {
		length uint32
	}
	buf := &bytes.Buffer{}
	tests := []struct {
		name    string
		fields  fields
		msgs    []string
		want    string
		wantErr bool
	}{
		{
			name:    "first",
			fields:  fields{length: 8},
			msgs:    []string{"12345678"},
			want:    "12345678",
			wantErr: false,
		},
		{
			name:    "second",
			fields:  fields{length: 7},
			msgs:    []string{"12345678"},
			want:    "",
			wantErr: true,
		},
		{
			name:    "third",
			fields:  fields{length: 8},
			msgs:    []string{"12345678", "87654321"},
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
				writer := framebuf.NewWriter()
				if err := c.Encode(writer, framebuf.NewFrame([]byte(msg))); (err != nil) != tt.wantErr {
					t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				} else if err == nil {
					if flushErr := writer.FlushTo(buf); flushErr != nil {
						t.Fatalf("FlushTo() error = %v", flushErr)
					}
				}
			})
		}
		if buf.String() != tt.want {
			t.Errorf("Encode() error, want = %s, got %s", tt.want, buf.String())
		}
		buf.Reset()
	}
}
