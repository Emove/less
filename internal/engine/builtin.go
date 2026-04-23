package engine

import (
	"context"

	"github.com/emove/less"
)

func NewRouterMiddleware(router less.Router) less.Middleware {
	return func(handler less.Handler) less.Handler {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			//if err := handler(ctx, ch, message);err != nil {
			//	return err
			//}
			h, err := router(ctx, ch, message)
			if err != nil {
				return err
			}
			return h(ctx, ch, message)
		}
	}
}
