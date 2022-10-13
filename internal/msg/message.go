package msg

const (
	MAGIC = 76698383
)

const (
	Call = iota + 1
	Reply
	Oneway
)

type LessMessage struct {
	Magic   uint32 `json:"magic"`
	MsgType uint16 `json:"msg_type"`
	Body    []byte `json:"body"`
}

func NewMessage(msgType uint16, body string) *LessMessage {
	return &LessMessage{
		Magic:   MAGIC,
		MsgType: msgType,
		Body:    []byte(body),
	}
}
