package payload

import (
	"bytes"
	"encoding/json"
	"github.com/emove/less/codec"
	"github.com/emove/less/pkg/io"
	"github.com/emove/less/pkg/io/reader"
	"github.com/emove/less/pkg/io/writer"
	"reflect"
	"testing"
)

type MyStruct struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Gender string `json:"gender"`
	Score  int    `json:"score"`
}

var (
	jsonBytes = []byte(`{"id":1,"name":"jason","gender":"male","score":100}`)
)

// BenchmarkJSONUnMarshalByMap-8   	  521718	      2121 ns/op	     864 B/op	      28 allocs/op
func BenchmarkJSONUnMarshalByMap(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := make(map[string]interface{})
		_ = json.Unmarshal(jsonBytes, &m)
	}
}

// BenchmarkJSONUnMarshalByType-8   	 1000000	      1014 ns/op	     264 B/op	       7 allocs/op
func BenchmarkJSONUnMarshalByType(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg := &MyStruct{}
		_ = json.Unmarshal(jsonBytes, msg)
	}
}

// BenchmarkJSONUnMarshalByNew-8   	 1000000	      1033 ns/op	     264 B/op	       7 allocs/op
func BenchmarkJSONUnMarshalByNew(b *testing.B) {
	t := parseType(MyStruct{})
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg := reflect.New(t).Interface()
		_ = json.Unmarshal(jsonBytes, msg)
	}

	//msg := reflect.New(t).Interface()
	//_ = json.Unmarshal(jsonBytes, msg)
	//fmt.Printf("%v\n", msg)
}

func Test_jsonPayloadCodec_Marshal(t *testing.T) {
	msg := &MyStruct{}
	_ = json.Unmarshal(jsonBytes, msg)
	buff := &bytes.Buffer{}
	type args struct {
		message interface{}
		writer  io.Writer
	}
	tests := []struct {
		name    string
		codec   codec.PayloadCodec
		args    args
		wantErr bool
	}{
		{
			name:    "first",
			codec:   NewJSONCodec(),
			args:    args{message: msg, writer: writer.NewBufferWriter(buff)},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.codec.Marshal(tt.args.message, tt.args.writer); (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			_ = tt.args.writer.Flush()
			if !reflect.DeepEqual(buff.Bytes(), jsonBytes) {
				t.Errorf("Marshal err, want: %s, got: %s", string(jsonBytes), buff.String())
			}
		})
	}
}

func Test_jsonPayloadCodec_UnMarshal(t *testing.T) {
	msg := &MyStruct{}
	_ = json.Unmarshal(jsonBytes, msg)
	type args struct {
		reader io.Reader
	}
	tests := []struct {
		name        string
		codec       codec.PayloadCodec
		args        args
		wantMessage interface{}
		wantErr     bool
	}{
		{
			name:        "byType",
			codec:       NewJSONCodecWithType(MyStruct{}),
			args:        args{reader: reader.NewLimitReader(reader.NewBufferReader(bytes.NewReader(jsonBytes)), uint32(len(jsonBytes)))},
			wantMessage: msg,
			wantErr:     false,
		},
		{
			name:  "byMap",
			codec: NewJSONCodec(),
			args:  args{reader: reader.NewLimitReader(reader.NewBufferReader(bytes.NewReader(jsonBytes)), uint32(len(jsonBytes)))},
			wantMessage: map[string]interface{}{
				"id": 1, "name": "jason", "gender": "male", "score": 100,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMessage, err := tt.codec.UnMarshal(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnMarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMessage, tt.wantMessage) {
				m1, o1 := gotMessage.(map[string]interface{})
				m2, o2 := tt.wantMessage.(map[string]interface{})
				if !o1 || !o2 {
					t.Errorf("UnMarshal() gotMessage = %v, want %v", gotMessage, tt.wantMessage)
				}
				if o1 && o2 && !isMapEq(m1, m2) {
					t.Errorf("UnMarshal() gotMessage = %v, want %v", gotMessage, tt.wantMessage)
				}
			}
			//t.Logf("%v", gotMessage)
		})
	}
}

func Test_parseType(t *testing.T) {
	type args struct {
		msg interface{}
	}
	tests := []struct {
		name string
		args args
		want reflect.Kind
	}{
		{
			name: "struct",
			args: args{msg: args{}},
			want: reflect.Struct,
		},
		{
			name: "ptr",
			args: args{msg: &args{}},
			want: reflect.Struct,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got reflect.Type
			if got = parseType(tt.args.msg); !reflect.DeepEqual(got.Kind(), tt.want) {
				t.Errorf("parseType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func isMapEq(m1, m2 map[string]interface{}) bool {
	if len(m1) != len(m2) {
		return false
	}
	//for k1, v1 := range m1 {
	//	if v2, ok := m2[k1]; !ok || !reflect.DeepEqual(v1, v2) {
	//		fmt.Printf("v1: %v, v2: %v", v1, v2)
	//		return false
	//	}
	//}
	//return true
	marshal1, _ := json.Marshal(m1)
	marshal2, _ := json.Marshal(m2)
	return string(marshal1) == string(marshal2)
}
