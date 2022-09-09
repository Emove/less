package context

import (
	"context"
	"github.com/emove/less/channel"
	"sync"
)

type Context interface {
	Context() context.Context

	SetContext(ctx context.Context)

	Channel() channel.Channel

	Set(key, value interface{})

	Get(key interface{}) (value interface{}, exists bool)
}

type ctx struct {
	ctx   context.Context
	ch    channel.Channel
	props sync.Map
}

var pool = sync.Pool{
	New: func() interface{} {
		return &ctx{}
	},
}

func New() *ctx {
	return pool.Get().(*ctx)
}

func (c *ctx) Context() context.Context {
	return c.ch.Context()
}

func (c *ctx) SetContext(cc context.Context) {
	c.ctx = cc
}

func (c *ctx) Channel() channel.Channel {
	return c.ch
}

func (c *ctx) Set(k, v interface{}) {
	c.props.Store(k, v)
}

func (c *ctx) Get(k interface{}) (interface{}, bool) {
	return c.props.Load(k)
}
