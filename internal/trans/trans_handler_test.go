package trans

import (
	"context"
	"testing"

	"github.com/emove/less"
)

func Test_newRouter(t *testing.T) {
	nilHandler := func(ctx context.Context, ch less.Channel, message interface{}) error {
		return nil
	}

	mw := newRouter(func(ctx context.Context, channel less.Channel, msg interface{}) (less.Handler, error) {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			t.Logf("router handler, handle message: %v", message)
			return nil
		}, nil
	})

	_ = mw(nilHandler)(context.Background(), nil, "router test")
}
