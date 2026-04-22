package payload

import (
	"encoding/json"
	"github.com/emove/less/internal/engine/framebuf"
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
}

func TestJSONCodec_MarshalReturnsFrame(t *testing.T) {
	msg := &MyStruct{}
	_ = json.Unmarshal(jsonBytes, msg)

	codec := NewJSONCodec().(*jsonPayloadCodec)
	got, err := codec.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	frame, ok := any(got).(framebuf.Frame)
	if !ok {
		t.Fatalf("Marshal() returned %T, want frame with Bytes() []byte", got)
	}

	if !reflect.DeepEqual(frame.Bytes(), jsonBytes) {
		t.Fatalf("Marshal() frame bytes = %s, want %s", string(frame.Bytes()), string(jsonBytes))
	}
}

func TestJSONCodec_UnmarshalConsumesFrame(t *testing.T) {
	codec := NewJSONCodecWithType(MyStruct{}).(*jsonPayloadCodec)

	frame := framebuf.NewFrame(append([]byte(nil), jsonBytes...))
	defer frame.Release()
	gotMessage, err := codec.Unmarshal(frame)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := &MyStruct{}
	_ = json.Unmarshal(jsonBytes, want)

	if !reflect.DeepEqual(gotMessage, want) {
		t.Fatalf("Unmarshal() gotMessage = %v, want %v", gotMessage, want)
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
