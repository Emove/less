package payload

import (
	"github.com/emove/less/internal/engine/framebuf"
	"reflect"
	"testing"
)

func TestTextCodec_MarshalReturnsFrame(t *testing.T) {
	te := &textPayloadCodec{}

	got, err := te.Marshal("hello world")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	frame, ok := any(got).(framebuf.Frame)
	if !ok {
		t.Fatalf("Marshal() returned %T, want frame with Bytes() []byte", got)
	}

	if !reflect.DeepEqual(frame.Bytes(), []byte("hello world")) {
		t.Fatalf("Marshal() frame bytes = %v, want %v", frame.Bytes(), []byte("hello world"))
	}
}

func TestTextCodec_UnmarshalConsumesFrame(t *testing.T) {
	te := &textPayloadCodec{}

	frame := framebuf.NewFrame([]byte("hello world"))
	defer frame.Release()
	gotMessage, err := te.Unmarshal(frame)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if !reflect.DeepEqual(gotMessage, "hello world") {
		t.Fatalf("Unmarshal() gotMessage = %v, want %v", gotMessage, "hello world")
	}
}
