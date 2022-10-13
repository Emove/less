package less

import (
	"context"
)

// Interceptor defines the interceptor used to intercept message.
type Interceptor func(message interface{}) bool

// Handler defines the handler invoked by Middleware.
type Handler func(ctx context.Context, ch Channel, message interface{}) error

// Middleware is transport Middleware.
type Middleware func(handler Handler) Handler

// Chain returns a Middleware that specifies the chained handler for transport.
func Chain(ms ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(ms) - 1; i >= 0; i-- {
			next = ms[i](next)
		}
		return next
	}
}
