package less

import (
	"context"
)

type Handler func(ctx context.Context, ch Channel, message interface{}) error

type Middleware func(handler Handler) Handler

func Chain(ms ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := len(ms) - 1; i >= 0; i-- {
			next = ms[i](next)
		}
		return next
	}
}
