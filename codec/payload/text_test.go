package payload

import (
	"reflect"
	"testing"
)

func TestTextPayloadCodec_Marshal(t *testing.T) {
	tests := []struct {
		message interface{}
		want    []byte
		wantErr bool
	}{
		{
			message: "hello world",
			want:    []byte("hello world"),
			wantErr: false,
		},
		{
			message: []byte("hello world"),
			want:    []byte("hello world"),
			wantErr: false,
		},
		{
			message: 1,
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			te := &textPayloadCodec{}
			got, err := te.Marshal(tt.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("Marshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Marshal() want = %v, got = %v", tt.want, got)
			}
		})
	}
}

func TestTextPayloadCodec_Unmarshal(t *testing.T) {
	tests := []struct {
		payload     []byte
		wantMessage interface{}
		wantErr     bool
	}{
		{
			payload:     []byte("hello world"),
			wantMessage: "hello world",
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			te := &textPayloadCodec{}
			gotMessage, err := te.Unmarshal(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotMessage, tt.wantMessage) {
				t.Errorf("Unmarshal() gotMessage = %v, want %v", gotMessage, tt.wantMessage)
			}
		})
	}
}
