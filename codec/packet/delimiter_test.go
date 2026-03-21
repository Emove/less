package packet

import (
	"bytes"
	"github.com/emove/less/io"
	ior "github.com/emove/less/io/reader"
	"github.com/emove/less/io/writer"
	"reflect"
	"testing"
)

func Test_delimiterCodec_Decode(t *testing.T) {
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		name        string
		times       int
		codec       *delimiterCodec
		args        args
		wantPayload [][]byte
		wantErr     bool
	}{
		{
			name:        "first",
			times:       1,
			codec:       NewDelimiterCodec("00", 9).(*delimiterCodec),
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("123456700")))},
			wantPayload: [][]byte{[]byte("1234567")},
			wantErr:     false,
		},
		{
			name:        "second",
			times:       2,
			codec:       NewDelimiterCodec("\n", 8).(*delimiterCodec),
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567\n7654321\n")))},
			wantPayload: [][]byte{[]byte("1234567"), []byte("7654321")},
			wantErr:     false,
		},
		{
			name:        "third",
			times:       1,
			codec:       NewDelimiterCodec("\n", 7).(*delimiterCodec),
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567\n")))},
			wantPayload: [][]byte{},
			wantErr:     true,
		},
		{
			name:        "forth",
			times:       1,
			codec:       NewDelimiterCodec("\n", 8, DisableStripDelimiter()).(*delimiterCodec),
			args:        args{reader: ior.NewBufferReader(newTestReader([]byte("1234567\n")))},
			wantPayload: [][]byte{[]byte("1234567\n")},
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		for i := 0; i < tt.times; i++ {
			t.Run(tt.name, func(t *testing.T) {
				gotPayload, err := tt.codec.Decode(tt.args.reader)
				if (err != nil) != tt.wantErr {
					t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if err == nil && !reflect.DeepEqual(gotPayload, tt.wantPayload[i]) {
					t.Errorf("Decode() gotPayload = %v, want %v", gotPayload, tt.wantPayload[i])
				}
			})
		}
	}
}

func Test_delimiterCodec_Encode(t *testing.T) {
	buff := &bytes.Buffer{}
	type args struct {
		writer io.Writer
	}
	tests := []struct {
		name    string
		codec   *delimiterCodec
		args    args
		msg     []string
		want    []byte
		wantErr bool
	}{
		{
			name:    "first",
			codec:   NewDelimiterCodec("\t", 8).(*delimiterCodec),
			args:    args{writer: writer.NewBufferWriter(buff)},
			msg:     []string{"1234567", "7654321"},
			want:    []byte("1234567\t7654321\t"),
			wantErr: false,
		},
		{
			name:    "second",
			codec:   NewDelimiterCodec("\t", 8, DisableAutoAppendDelimiter()).(*delimiterCodec),
			args:    args{writer: writer.NewBufferWriter(buff)},
			msg:     []string{"1234567\t", "7654321\t"},
			want:    []byte("1234567\t7654321\t"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		for _, msg := range tt.msg {
			t.Run(tt.name, func(t *testing.T) {
				if err := tt.codec.Encode([]byte(msg), tt.args.writer); (err != nil) != tt.wantErr {
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
