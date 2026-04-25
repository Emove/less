package protocol

const (
	Version1 byte = 1
)

const (
	TypeAuth byte = iota + 1
	TypeAuthAck
	TypeHeartbeat
	TypeTelemetry
	TypeCommand
	TypeCommandAck
)

const HeaderSize = 10

type Header struct {
	Version    byte
	Type       byte
	RequestID  uint32
	BodyLength uint32
}

type AuthMessage struct {
	Version  byte
	DeviceID string
	Secret   string
}

type AuthAckMessage struct {
	Version byte
	Status  string
}

type HeartbeatMessage struct {
	Version   byte
	Timestamp int64
}

type TelemetryMessage struct {
	Version byte
	Metrics map[string]string
}

type CommandMessage struct {
	Version   byte
	RequestID uint32
	Name      string
}

type CommandAckMessage struct {
	Version   byte
	RequestID uint32
	Status    string
}

func Auth(deviceID, secret string) *AuthMessage {
	return &AuthMessage{Version: Version1, DeviceID: deviceID, Secret: secret}
}

func AuthAck(status string) *AuthAckMessage {
	return &AuthAckMessage{Version: Version1, Status: status}
}

func Heartbeat(ts int64) *HeartbeatMessage {
	return &HeartbeatMessage{Version: Version1, Timestamp: ts}
}

func Telemetry(metrics map[string]string) *TelemetryMessage {
	cp := make(map[string]string, len(metrics))
	for key, value := range metrics {
		cp[key] = value
	}
	return &TelemetryMessage{Version: Version1, Metrics: cp}
}

func Command(requestID uint32, name string) *CommandMessage {
	return &CommandMessage{Version: Version1, RequestID: requestID, Name: name}
}

func CommandAck(requestID uint32, status string) *CommandAckMessage {
	return &CommandAckMessage{Version: Version1, RequestID: requestID, Status: status}
}
