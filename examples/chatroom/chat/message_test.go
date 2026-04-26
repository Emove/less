package chat

import (
	"reflect"
	"testing"
)

func TestMessageConstructors(t *testing.T) {
	tests := []struct {
		name string
		got  *Message
		want *Message
	}{
		{
			name: "set name",
			got:  SetName("alice"),
			want: &Message{Type: TypeSetName, Name: "alice"},
		},
		{
			name: "chat",
			got:  Chat("alice", "hello"),
			want: &Message{Type: TypeChat, Name: "alice", Text: "hello"},
		},
		{
			name: "system",
			got:  System("alice joined"),
			want: &Message{Type: TypeSystem, Text: "alice joined"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("message = %#v, want %#v", tt.got, tt.want)
			}
		})
	}
}

func TestJSONCodecRoundTrip(t *testing.T) {
	codec := NewJSONCodec()

	frame, err := codec.Marshal(Chat("alice", "hello"))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	defer frame.Release()

	got, err := codec.Unmarshal(frame)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	want := &Message{Type: TypeChat, Name: "alice", Text: "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unmarshal() = %#v, want %#v", got, want)
	}
}
