package middleware

type HandlerOptions struct {
	InboundHandlers  []Handler
	OutboundHandlers []Handler
}
