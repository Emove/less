package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/emove/less/codec"
)

var (
	ErrShortHeader        = errors.New("protocol header too short")
	ErrUnsupportedVersion = errors.New("unsupported protocol version")
	ErrUnknownType        = errors.New("unknown protocol message type")
	ErrBodyLengthMismatch = errors.New("protocol body length mismatch")
	ErrMalformedBody      = errors.New("malformed protocol body")
	ErrUnsupportedMessage = errors.New("unsupported protocol message")
)

type payloadCodec struct{}

var _ codec.PayloadCodec = (*payloadCodec)(nil)

func NewCodec() codec.PayloadCodec {
	return &payloadCodec{}
}

func (*payloadCodec) Name() string {
	return "device-gateway-payload-codec"
}

func (*payloadCodec) Marshal(message any) (codec.Frame, error) {
	header, body, err := buildPayload(message)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, HeaderSize+len(body))
	buf[0] = header.Version
	buf[1] = header.Type
	binary.BigEndian.PutUint32(buf[2:6], header.RequestID)
	binary.BigEndian.PutUint32(buf[6:10], uint32(len(body)))
	copy(buf[10:], body)

	return NewFrame(buf), nil
}

func (*payloadCodec) Unmarshal(payload codec.Frame) (any, error) {
	if payload == nil {
		return nil, ErrShortHeader
	}

	raw := payload.Bytes()
	if len(raw) < HeaderSize {
		return nil, ErrShortHeader
	}

	header := Header{
		Version:    raw[0],
		Type:       raw[1],
		RequestID:  binary.BigEndian.Uint32(raw[2:6]),
		BodyLength: binary.BigEndian.Uint32(raw[6:10]),
	}
	if header.Version != Version1 {
		return nil, ErrUnsupportedVersion
	}

	body := raw[HeaderSize:]
	if uint32(len(body)) != header.BodyLength {
		return nil, ErrBodyLengthMismatch
	}

	values, err := decodeBody(body)
	if err != nil {
		return nil, err
	}

	return decodeMessage(header, values)
}

func buildPayload(message any) (Header, []byte, error) {
	switch msg := message.(type) {
	case *AuthMessage:
		if msg == nil {
			return Header{}, nil, fmt.Errorf("%w: nil *AuthMessage", ErrUnsupportedMessage)
		}
		version, err := normalizeVersion(msg.Version)
		if err != nil {
			return Header{}, nil, err
		}
		body, err := encodeBody(map[string]string{
			"device_id": msg.DeviceID,
			"secret":    msg.Secret,
		})
		if err != nil {
			return Header{}, nil, err
		}
		return Header{Version: version, Type: TypeAuth, BodyLength: uint32(len(body))}, body, nil
	case *AuthAckMessage:
		if msg == nil {
			return Header{}, nil, fmt.Errorf("%w: nil *AuthAckMessage", ErrUnsupportedMessage)
		}
		version, err := normalizeVersion(msg.Version)
		if err != nil {
			return Header{}, nil, err
		}
		body, err := encodeBody(map[string]string{"status": msg.Status})
		if err != nil {
			return Header{}, nil, err
		}
		return Header{Version: version, Type: TypeAuthAck, BodyLength: uint32(len(body))}, body, nil
	case *HeartbeatMessage:
		if msg == nil {
			return Header{}, nil, fmt.Errorf("%w: nil *HeartbeatMessage", ErrUnsupportedMessage)
		}
		version, err := normalizeVersion(msg.Version)
		if err != nil {
			return Header{}, nil, err
		}
		body, err := encodeBody(map[string]string{"ts": strconv.FormatInt(msg.Timestamp, 10)})
		if err != nil {
			return Header{}, nil, err
		}
		return Header{Version: version, Type: TypeHeartbeat, BodyLength: uint32(len(body))}, body, nil
	case *TelemetryMessage:
		if msg == nil {
			return Header{}, nil, fmt.Errorf("%w: nil *TelemetryMessage", ErrUnsupportedMessage)
		}
		version, err := normalizeVersion(msg.Version)
		if err != nil {
			return Header{}, nil, err
		}
		body, err := encodeBody(msg.Metrics)
		if err != nil {
			return Header{}, nil, err
		}
		return Header{Version: version, Type: TypeTelemetry, BodyLength: uint32(len(body))}, body, nil
	case *CommandMessage:
		if msg == nil {
			return Header{}, nil, fmt.Errorf("%w: nil *CommandMessage", ErrUnsupportedMessage)
		}
		version, err := normalizeVersion(msg.Version)
		if err != nil {
			return Header{}, nil, err
		}
		body, err := encodeBody(map[string]string{"name": msg.Name})
		if err != nil {
			return Header{}, nil, err
		}
		return Header{
			Version:    version,
			Type:       TypeCommand,
			RequestID:  msg.RequestID,
			BodyLength: uint32(len(body)),
		}, body, nil
	case *CommandAckMessage:
		if msg == nil {
			return Header{}, nil, fmt.Errorf("%w: nil *CommandAckMessage", ErrUnsupportedMessage)
		}
		version, err := normalizeVersion(msg.Version)
		if err != nil {
			return Header{}, nil, err
		}
		body, err := encodeBody(map[string]string{"status": msg.Status})
		if err != nil {
			return Header{}, nil, err
		}
		return Header{
			Version:    version,
			Type:       TypeCommandAck,
			RequestID:  msg.RequestID,
			BodyLength: uint32(len(body)),
		}, body, nil
	default:
		return Header{}, nil, fmt.Errorf("%w: %T", ErrUnsupportedMessage, message)
	}
}

func normalizeVersion(version byte) (byte, error) {
	if version == 0 {
		return Version1, nil
	}
	if version != Version1 {
		return 0, ErrUnsupportedVersion
	}
	return version, nil
}

func decodeMessage(header Header, values map[string]string) (any, error) {
	switch header.Type {
	case TypeAuth:
		deviceID, ok := values["device_id"]
		if !ok || deviceID == "" {
			return nil, ErrMalformedBody
		}
		secret, ok := values["secret"]
		if !ok || secret == "" {
			return nil, ErrMalformedBody
		}
		return &AuthMessage{Version: header.Version, DeviceID: deviceID, Secret: secret}, nil
	case TypeAuthAck:
		status, ok := values["status"]
		if !ok || status == "" {
			return nil, ErrMalformedBody
		}
		return &AuthAckMessage{Version: header.Version, Status: status}, nil
	case TypeHeartbeat:
		rawTimestamp, ok := values["ts"]
		if !ok || rawTimestamp == "" {
			return nil, ErrMalformedBody
		}
		timestamp, err := strconv.ParseInt(rawTimestamp, 10, 64)
		if err != nil {
			return nil, ErrMalformedBody
		}
		return &HeartbeatMessage{Version: header.Version, Timestamp: timestamp}, nil
	case TypeTelemetry:
		if len(values) == 0 {
			return nil, ErrMalformedBody
		}
		return &TelemetryMessage{Version: header.Version, Metrics: values}, nil
	case TypeCommand:
		name, ok := values["name"]
		if !ok || name == "" {
			return nil, ErrMalformedBody
		}
		return &CommandMessage{Version: header.Version, RequestID: header.RequestID, Name: name}, nil
	case TypeCommandAck:
		status, ok := values["status"]
		if !ok || status == "" {
			return nil, ErrMalformedBody
		}
		return &CommandAckMessage{Version: header.Version, RequestID: header.RequestID, Status: status}, nil
	default:
		return nil, ErrUnknownType
	}
}

func encodeBody(values map[string]string) ([]byte, error) {
	if len(values) == 0 {
		return nil, nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		if err := validateBodyToken(key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if err := validateBodyToken(values[key]); err != nil {
			return nil, err
		}
		parts = append(parts, key+"="+values[key])
	}

	return []byte(strings.Join(parts, ";")), nil
}

func validateBodyToken(value string) error {
	if strings.ContainsAny(value, ";=") {
		return fmt.Errorf("%w: reserved delimiter in %q", ErrMalformedBody, value)
	}
	return nil
}

func decodeBody(body []byte) (map[string]string, error) {
	if len(body) == 0 {
		return map[string]string{}, nil
	}

	values := make(map[string]string, len(body))
	for _, part := range strings.Split(string(body), ";") {
		pair := strings.SplitN(part, "=", 2)
		if len(pair) != 2 || pair[0] == "" || pair[1] == "" {
			return nil, ErrMalformedBody
		}
		values[pair[0]] = pair[1]
	}

	return values, nil
}
