package transport

type Context interface {
	Transporter() Transporter
	Message() Message
}

func NewContext(tsp Transporter, msg Message) Context {
	return &context{
		tsp: tsp,
		msg: msg,
	}
}

type context struct {
	tsp Transporter
	msg Message
}

func (r *context) Transporter() Transporter {
	return r.tsp
}

func (r *context) Message() Message {
	return r.msg
}
