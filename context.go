package less

import (
	"context"
	"sync"
)

type Context interface {
	Context() context.Context

	SetContext(ctx context.Context)

	Channel() Channel

	Set(key, value interface{})

	Get(key interface{}) (value interface{}, exists bool)
}

type ctx struct {
	ctx   context.Context
	ch    Channel
	props map[interface{}]interface{}
}

var pool = sync.Pool{
	New: func() interface{} {
		return &ctx{}
	},
}

func New(ch Channel) *ctx {
	c := pool.Get().(*ctx)
	c.ch = ch
	// channel context by default
	c.ctx = ch.Context()
	return c
}

func (c *ctx) Context() context.Context {
	return c.ctx
}

func (c *ctx) SetContext(cc context.Context) {
	c.ctx = cc
}

func (c *ctx) Channel() Channel {
	return c.ch
}

func (c *ctx) Set(k, v interface{}) {
	c.props[k] = v
}

func (c *ctx) Get(k interface{}) (v interface{}, ok bool) {
	v, ok = c.props[k]
	return
}

func (c *ctx) Release() {
	c.props = nil
	c.ctx = nil
}
