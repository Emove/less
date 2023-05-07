package transport

import (
	"context"
	"github.com/emove/less"
	"github.com/emove/less/internal/channel"
	"github.com/emove/less/router"
)

func OutboundHandler(hdr BoundHandler) less.Handler {
	return func(ctx context.Context, ch less.Channel, message interface{}) error {
		return hdr.OnWrite(ch.(*channel.Channel), message)
	}
}

func NewRouterMiddleware(router router.Router) less.Middleware {
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
