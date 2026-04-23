package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/emove/less/codec"
	"github.com/emove/less/internal/engine/framebuf"
	"io"
	"reflect"
	"testing"
)

func TestVariableLengthCodec_DecodeReturnsFrame(t *testing.T) {
	va := &variableLengthCodec{}

	gotPayload, err := va.Decode(reader([]byte("hello world")))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	defer gotPayload.Release()

	if !reflect.DeepEqual(gotPayload.Bytes(), []byte("hello world")) {
		t.Fatalf("Decode() frame bytes = %v, want %v", gotPayload.Bytes(), []byte("hello world"))
	}
}

func TestVariableLengthCodec_Encode(t *testing.T) {
	type args struct {
		payload []byte
	}
	buff := &bytes.Buffer{}
	tests := []struct {
		args    args
		want    string
		wantErr bool
	}{
		{
			args:    args{payload: []byte("hello world")},
			want:    "hello world",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			va := &variableLengthCodec{}
			writer := framebuf.NewWriter()
			if err := va.Encode(writer, framebuf.NewFrame(tt.args.payload)); (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			} else if err == nil {
				if flushErr := writer.FlushTo(buff); flushErr != nil {
					t.Fatalf("FlushTo() error = %v", flushErr)
				}
			}
			got := buff.Bytes()[binary.MaxVarintLen32:]
			if string(got) != tt.want {
				t.Errorf("Encode() want = %v, got = %v", tt.want, string(got))
			}
			buff.Reset()
		})
	}
}

func reader(msg []byte) codec.ReaderBuffer {
	buff := &bytes.Buffer{}
	header := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(header, uint32(len(msg)))
	buff.Write(header)
	buff.Write(msg)
	return framebuf.NewReader(buff)
}

type chunkedBytesReader struct {
	chunks [][]byte
	index  int
}

func (r *chunkedBytesReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}
	chunk := r.chunks[r.index]
	r.index++
	return copy(p, chunk), nil
}

func TestVariableLengthCodec_DecodeFragmentedHeaderAndBody(t *testing.T) {
	reader := framebuf.NewReader(&chunkedBytesReader{
		chunks: [][]byte{
			{0},
			{0},
			{0},
			{5},
			{'h'},
			{'e', 'l'},
			{'l', 'o'},
		},
	})

	frame, err := NewVariableLengthCodec().Decode(reader)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	defer frame.Release()

	if got := string(frame.Bytes()); got != "hello" {
		t.Fatalf("frame.Bytes() = %q, want %q", got, "hello")
	}
}
