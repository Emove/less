package router

import (
	"context"
	"github.com/emove/less"
)

// Router defines router func, the ctx is which be returned on OnChannel hook
type Router func(ctx context.Context, channel less.Channel, msg interface{}) (less.Handler, error)
