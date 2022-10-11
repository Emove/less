package channel

import (
	"context"
	"sync"

	"github.com/emove/less"
)

// PipelineFactory is a factory to create Pipeline.
type PipelineFactory func(ch *Channel) *pipeline

var pool = sync.Pool{}

// NewPipelineFactory returns a pipeline factory.
func NewPipelineFactory(
	onChannel []less.OnChannel, onChannelClosed []less.OnChannelClosed,
	inbound []less.Middleware, outbound []less.Middleware,
	router less.Middleware, outboundHandler less.Handler,
) PipelineFactory {
	pool.New = func() interface{} {
		return &pipeline{
			onChannelChain:       onChannel,
			onChannelClosedChain: onChannelClosed,
			inbound:              inbound,
			outbound:             outbound,
			router:               router,
			outboundHandler:      outboundHandler,
		}
	}
	return func(ch *Channel) *pipeline {
		pl := pool.Get().(*pipeline)
		pl.ch = ch
		return pl
	}
}

type pipeline struct {
	onChannelChain       []less.OnChannel
	onChannelClosedChain []less.OnChannelClosed
	inbound              []less.Middleware
	outbound             []less.Middleware
	router               less.Middleware
	outboundHandler      less.Handler

	ch    *Channel
	chocc []less.OnChannelClosed
	chIn  []less.Middleware
	chOut []less.Middleware
}

// AddOnChannelClosed adds channel's specific OnChannelClosed hooks
func (pl *pipeline) AddOnChannelClosed(onChannelClosed ...less.OnChannelClosed) {
	if len(onChannelClosed) > 0 {
		pl.chocc = append(pl.chocc, onChannelClosed...)
	}
}

// AddInbound adds channel's specific Inbound middlewares
func (pl *pipeline) AddInbound(inbound ...less.Middleware) {
	if len(inbound) > 0 {
		pl.chIn = append(pl.chIn, inbound...)
	}
}

// AddOutbound adds channel's specific outbound middlewares
func (pl *pipeline) AddOutbound(outbound ...less.Middleware) {
	if len(outbound) > 0 {
		pl.chOut = append(pl.chOut, outbound...)
	}
}

// FireOnChannel fires OnChannel hooks
func (pl *pipeline) FireOnChannel(ctx context.Context) (err error) {
	pl.ch.SetContext(ctx)
	for _, onChannel := range pl.onChannelChain {
		ctx, err = onChannel(ctx, pl.ch)
		if err != nil {
			return err
		}
		if ctx != nil {
			pl.ch.SetContext(ctx)
		}
	}
	pl.ch.active()
	return nil
}

// FireOnChannelClosed fires common onChannelClosed hooks and channel's specific OnChannelClosed hooks
func (pl *pipeline) FireOnChannelClosed(err error) {
	onChannelClosedChain := append(pl.onChannelClosedChain, pl.chocc...)
	for _, onChannelClosed := range onChannelClosedChain {
		onChannelClosed(pl.ch.Context(), pl.ch, err)
	}
}

// FireInbound fires common inbound middlewares and channel's specific inbound middlewares
func (pl *pipeline) FireInbound(message interface{}) error {

	ch := pl.ch
	mws := less.Chain(less.Chain(pl.inbound...), less.Chain(pl.chIn...))

	if pl.router != nil {
		mws = less.Chain(mws, pl.router)
	}

	return mws(emptyHandler)(ch.Context(), pl.ch, message)
}

// FireOutbound fires common outbound middlewares and channel's specific outbound middlewares
func (pl *pipeline) FireOutbound(message interface{}) error {
	ch := pl.ch
	mws := less.Chain(less.Chain(pl.chOut...), less.Chain(pl.outbound...))
	handler := pl.outboundHandler

	if handler == nil {
		handler = emptyHandler
	}

	return mws(handler)(ch.Context(), ch, message)
}

// Release releases channel's specific hooks and reuse pipeline
func (pl *pipeline) Release() {
	pl.ch = nil
	pl.chocc = nil
	pl.chIn = nil
	pl.chOut = nil

	pool.Put(pl)
}

func emptyHandler(_ context.Context, _ less.Channel, _ interface{}) error { return nil }
