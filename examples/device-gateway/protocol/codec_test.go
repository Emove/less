package protocol

import (
	"errors"
	"reflect"
	"testing"
)

func TestCodecMarshalUnmarshalRoundTrip(t *testing.T) {
	codec := NewCodec()

	tests := []struct {
		name    string
		message any
		want    any
	}{
		{
			name:    "auth",
			message: Auth("dev-001", "demo-secret"),
			want:    &AuthMessage{Version: Version1, DeviceID: "dev-001", Secret: "demo-secret"},
		},
		{
			name:    "heartbeat",
			message: Heartbeat(1714032000),
			want:    &HeartbeatMessage{Version: Version1, Timestamp: 1714032000},
		},
		{
			name:    "telemetry",
			message: Telemetry(map[string]string{"humidity": "48", "temp": "23.4"}),
			want:    &TelemetryMessage{Version: Version1, Metrics: map[string]string{"humidity": "48", "temp": "23.4"}},
		},
		{
			name:    "command",
			message: Command(7, "reboot"),
			want:    &CommandMessage{Version: Version1, RequestID: 7, Name: "reboot"},
		},
		{
			name:    "command_ack",
			message: CommandAck(7, "ok"),
			want:    &CommandAckMessage{Version: Version1, RequestID: 7, Status: "ok"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := codec.Marshal(tt.message)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			defer frame.Release()

			got, err := codec.Unmarshal(frame)
			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Unmarshal() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCodecRejectsMalformedPayloads(t *testing.T) {
	codec := NewCodec()

	tests := []struct {
		name    string
		payload []byte
		wantErr error
	}{
		{
			name:    "unsupported_version",
			payload: []byte{9, TypeAuth, 0, 0, 0, 0, 0, 0, 0, 0},
			wantErr: ErrUnsupportedVersion,
		},
		{
			name:    "unknown_type",
			payload: []byte{Version1, 99, 0, 0, 0, 0, 0, 0, 0, 0},
			wantErr: ErrUnknownType,
		},
		{
			name:    "short_header",
			payload: []byte{Version1, TypeAuth},
			wantErr: ErrShortHeader,
		},
		{
			name:    "body_length_mismatch",
			payload: append([]byte{Version1, TypeAuth, 0, 0, 0, 0, 0, 0, 0, 5}, []byte("a=1")...),
			wantErr: ErrBodyLengthMismatch,
		},
		{
			name:    "malformed_body",
			payload: append([]byte{Version1, TypeAuth, 0, 0, 0, 0, 0, 0, 0, 3}, []byte("bad")...),
			wantErr: ErrMalformedBody,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := codec.Unmarshal(NewFrame(tt.payload))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Unmarshal() error = %v, want %v", err, tt.wantErr)
			}
			if got != nil {
				t.Fatalf("Unmarshal() message = %#v, want nil", got)
			}
		})
	}
}

func TestEncodeBodySortsKeysForStableOutput(t *testing.T) {
	got, err := encodeBody(map[string]string{
		"temp":     "23.4",
		"humidity": "48",
	})
	if err != nil {
		t.Fatalf("encodeBody() error = %v", err)
	}

	if string(got) != "humidity=48;temp=23.4" {
		t.Fatalf("encodeBody() = %q, want %q", got, "humidity=48;temp=23.4")
	}
}

func TestCodecMarshalTypedNilDoesNotPanic(t *testing.T) {
	codec := NewCodec()

	var message *AuthMessage

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Marshal() panicked: %v", recovered)
		}
	}()

	frame, err := codec.Marshal(message)
	if !errors.Is(err, ErrUnsupportedMessage) {
		t.Fatalf("Marshal() error = %v, want %v", err, ErrUnsupportedMessage)
	}
	if frame != nil {
		t.Fatalf("Marshal() frame = %#v, want nil", frame)
	}
}

func TestCodecMarshalRejectsReservedDelimiters(t *testing.T) {
	codec := NewCodec()

	tests := []struct {
		name    string
		message any
	}{
		{
			name:    "auth_device_id_contains_semicolon",
			message: Auth("dev;001", "demo-secret"),
		},
		{
			name:    "auth_secret_contains_equals",
			message: Auth("dev-001", "demo=secret"),
		},
		{
			name:    "command_name_contains_semicolon",
			message: Command(7, "reboot;now"),
		},
		{
			name:    "command_ack_status_contains_equals",
			message: CommandAck(7, "ok=done"),
		},
		{
			name:    "telemetry_key_contains_equals",
			message: Telemetry(map[string]string{"temp=c": "23.4"}),
		},
		{
			name:    "telemetry_value_contains_semicolon",
			message: Telemetry(map[string]string{"temp": "23;4"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := codec.Marshal(tt.message)
			if !errors.Is(err, ErrMalformedBody) {
				t.Fatalf("Marshal() error = %v, want %v", err, ErrMalformedBody)
			}
			if frame != nil {
				t.Fatalf("Marshal() frame = %#v, want nil", frame)
			}
		})
	}
}
