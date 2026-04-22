package packet

import (
	"bytes"
	"github.com/emove/less/internal/engine/framebuf"
	"reflect"
	"testing"
)

func TestDelimiterCodec_DecodeReturnsRetainedFrame(t *testing.T) {
	codec := NewDelimiterCodec("\n", 8).(*delimiterCodec)

	gotPayload, err := codec.Decode(framebuf.NewReader(newTestReader([]byte("1234567\n"))))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	defer gotPayload.Release()

	if !reflect.DeepEqual(gotPayload.Bytes(), []byte("1234567")) {
		t.Fatalf("Decode() frame bytes = %v, want %v", gotPayload.Bytes(), []byte("1234567"))
	}
}

func TestDelimiterCodec_DecodeRetainsDelimiterWhenConfigured(t *testing.T) {
	codec := NewDelimiterCodec("\n", 8, DisableStripDelimiter()).(*delimiterCodec)

	gotPayload, err := codec.Decode(framebuf.NewReader(newTestReader([]byte("1234567\n"))))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	defer gotPayload.Release()

	if !reflect.DeepEqual(gotPayload.Bytes(), []byte("1234567\n")) {
		t.Fatalf("Decode() frame bytes = %v, want %v", gotPayload.Bytes(), []byte("1234567\n"))
	}
}

func Test_delimiterCodec_Encode(t *testing.T) {
	buff := &bytes.Buffer{}
	tests := []struct {
		name    string
		codec   *delimiterCodec
		msg     []string
		want    []byte
		wantErr bool
	}{
		{
			name:    "first",
			codec:   NewDelimiterCodec("\t", 8).(*delimiterCodec),
			msg:     []string{"1234567", "7654321"},
			want:    []byte("1234567\t7654321\t"),
			wantErr: false,
		},
		{
			name:    "second",
			codec:   NewDelimiterCodec("\t", 8, DisableAutoAppendDelimiter()).(*delimiterCodec),
			msg:     []string{"1234567\t", "7654321\t"},
			want:    []byte("1234567\t7654321\t"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		for _, msg := range tt.msg {
			t.Run(tt.name, func(t *testing.T) {
				writer := framebuf.NewWriter()
				if err := tt.codec.Encode(writer, framebuf.NewFrame([]byte(msg))); (err != nil) != tt.wantErr {
					t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				} else if err == nil {
					if flushErr := writer.FlushTo(buff); flushErr != nil {
						t.Fatalf("FlushTo() error = %v", flushErr)
					}
				}
			})
		}

		if buff.String() != string(tt.want) {
			t.Errorf("Encode() error want: %s, got: %s", buff.String(), string(tt.want))
		}
		buff.Reset()
	}
}
