package packet

import (
	"bytes"
	"github.com/emove/less/codec"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/pkg/io"
	ior "github.com/emove/less/pkg/io/reader"
	"github.com/emove/less/pkg/io/writer"
	"reflect"
	"testing"
)

func Test_delimiterCodec_Decode(t *testing.T) {
	type args struct {
		reader       io.Reader
		payloadCodec codec.PayloadCodec
	}
	tests := []struct {
		name        string
		times       int
		codec       codec.PacketCodec
		args        args
		wantMessage []string
		wantErr     bool
	}{
		{
			name:        "first",
			times:       1,
			codec:       NewDelimiterCodec("\n", 8),
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567\n"))), payloadCodec: payload.NewTextCodec()},
			wantMessage: []string{"1234567"},
			wantErr:     false,
		},
		{
			name:        "second",
			times:       2,
			codec:       NewDelimiterCodec("\n", 8),
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567\n7654321\n"))), payloadCodec: payload.NewTextCodec()},
			wantMessage: []string{"1234567", "7654321"},
			wantErr:     false,
		},
		{
			name:        "third",
			times:       1,
			codec:       NewDelimiterCodec("\n", 7),
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567\n"))), payloadCodec: payload.NewTextCodec()},
			wantMessage: []string{},
			wantErr:     true,
		},
		{
			name:        "forth",
			times:       1,
			codec:       NewDelimiterCodec("\n", 8, DisableStripDelimiter(false)),
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567\n"))), payloadCodec: payload.NewTextCodec()},
			wantMessage: []string{"1234567\n"},
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		for i := 0; i < tt.times; i++ {
			t.Run(tt.name, func(t *testing.T) {
				gotMessage, err := tt.codec.Decode(tt.args.reader, tt.args.payloadCodec)
				if (err != nil) != tt.wantErr {
					t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if err == nil && !reflect.DeepEqual(gotMessage, tt.wantMessage[i]) {
					t.Errorf("Decode() gotMessage = %v, want %v", gotMessage, tt.wantMessage[i])
				}
			})
		}
	}
}

func Test_delimiterCodec_Encode(t *testing.T) {
	buff := &bytes.Buffer{}
	type args struct {
		writer       io.Writer
		payloadCodec codec.PayloadCodec
	}
	tests := []struct {
		name    string
		codec   codec.PacketCodec
		args    args
		msg     []string
		want    []byte
		wantErr bool
	}{
		{
			name:    "first",
			codec:   NewDelimiterCodec("\t", 8),
			args:    args{writer: writer.NewBufferWriter(buff), payloadCodec: payload.NewTextCodec()},
			msg:     []string{"1234567", "7654321"},
			want:    []byte("1234567\t7654321\t"),
			wantErr: false,
		},
		{
			name:    "second",
			codec:   NewDelimiterCodec("\t", 8, DisableAutoAppendDelimiter(false)),
			args:    args{writer: writer.NewBufferWriter(buff), payloadCodec: payload.NewTextCodec()},
			msg:     []string{"1234567\t", "7654321\t"},
			want:    []byte("1234567\t7654321\t"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		for _, msg := range tt.msg {
			t.Run(tt.name, func(t *testing.T) {
				if err := tt.codec.Encode(msg, tt.args.writer, tt.args.payloadCodec); (err != nil) != tt.wantErr {
					t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}

		if buff.String() != string(tt.want) {
			t.Errorf("Encode() error want: %s, got: %s", buff.String(), string(tt.want))
		}
		buff.Reset()
	}
}
