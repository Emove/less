package payload

import (
	"encoding/json"
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

func Test_jsonPayloadCodec_Marshal(t *testing.T) {
	msg := &MyStruct{}
	_ = json.Unmarshal(jsonBytes, msg)
	tests := []struct {
		name    string
		codec   *jsonPayloadCodec
		message interface{}
		wantErr bool
	}{
		{
			name:    "first",
			codec:   NewJSONCodec().(*jsonPayloadCodec),
			message: msg,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.codec.Marshal(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, jsonBytes) {
				t.Errorf("Marshal err, want: %s, got: %s", string(jsonBytes), string(got))
			}
		})
	}
}

func Test_jsonPayloadCodec_Unmarshal(t *testing.T) {
	msg := &MyStruct{}
	_ = json.Unmarshal(jsonBytes, msg)
	tests := []struct {
		name        string
		codec       *jsonPayloadCodec
		payload     []byte
		wantMessage interface{}
		wantErr     bool
	}{
		{
			name:        "byType",
			codec:       NewJSONCodecWithType(MyStruct{}).(*jsonPayloadCodec),
			payload:     jsonBytes,
			wantMessage: msg,
			wantErr:     false,
		},
		{
			name:    "byMap",
			codec:   NewJSONCodec().(*jsonPayloadCodec),
			payload: jsonBytes,
			wantMessage: map[string]interface{}{
				"id": 1, "name": "jason", "gender": "male", "score": 100,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMessage, err := tt.codec.Unmarshal(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMessage, tt.wantMessage) {
				m1, o1 := gotMessage.(map[string]interface{})
				m2, o2 := tt.wantMessage.(map[string]interface{})
				if !o1 || !o2 {
					t.Errorf("Unmarshal() gotMessage = %v, want %v", gotMessage, tt.wantMessage)
				}
				if o1 && o2 && !isMapEq(m1, m2) {
					t.Errorf("Unmarshal() gotMessage = %v, want %v", gotMessage, tt.wantMessage)
				}
			}
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
	marshal1, _ := json.Marshal(m1)
	marshal2, _ := json.Marshal(m2)
	return string(marshal1) == string(marshal2)
}
