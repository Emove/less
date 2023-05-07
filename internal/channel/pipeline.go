package channel

import (
	"context"
	"sync"

	"github.com/emove/less"
)

// PipelineFactory is a factory to create Pipeline.
type PipelineFactory func() *pipeline

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
	return func() *pipeline {
		pl := pool.Get().(*pipeline)
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

	chocc []less.OnChannelClosed
	chIn  []less.Middleware
	chOut []less.Middleware
}

func (pl *pipeline) OnRead(ch *Channel, msg interface{}) (err error) {
	mws := less.Chain(less.Chain(pl.inbound...), less.Chain(pl.chIn...))

	if pl.router != nil {
		mws = less.Chain(mws, pl.router)
	}

	return mws(emptyHandler)(ch.Context(), ch, msg)
}

func (pl *pipeline) OnWrite(ch *Channel, msg interface{}) error {
	mws := less.Chain(less.Chain(pl.chOut...), less.Chain(pl.outbound...))
	handler := pl.outboundHandler

	if handler == nil {
		handler = emptyHandler
	}

	return mws(handler)(ch.Context(), ch, msg)
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
func (pl *pipeline) FireOnChannel(ch *Channel, ctx context.Context) (err error) {
	ch.SetContext(ctx)
	for _, onChannel := range pl.onChannelChain {
		ctx, err = onChannel(ctx, ch)
		if err != nil {
			return err
		}
		if ctx != nil {
			ch.SetContext(ctx)
		}
	}
	return nil
}

// FireOnChannelClosed fires common onChannelClosed hooks and channel's specific OnChannelClosed hooks
func (pl *pipeline) FireOnChannelClosed(ch *Channel, err error) {
	onChannelClosedChain := append(pl.onChannelClosedChain, pl.chocc...)
	for _, onChannelClosed := range onChannelClosedChain {
		onChannelClosed(ch.Context(), ch, err)
	}
}

// FireInbound fires common inbound middlewares and channel's specific inbound middlewares
func (pl *pipeline) FireInbound(ch *Channel, message interface{}) error {

	mws := less.Chain(less.Chain(pl.inbound...), less.Chain(pl.chIn...))

	if pl.router != nil {
		mws = less.Chain(mws, pl.router)
	}

	return mws(emptyHandler)(ch.Context(), ch, message)
}

// FireOutbound fires common outbound middlewares and channel's specific outbound middlewares
func (pl *pipeline) FireOutbound(ch *Channel, message interface{}) error {

	mws := less.Chain(less.Chain(pl.chOut...), less.Chain(pl.outbound...))
	handler := pl.outboundHandler

	if handler == nil {
		handler = emptyHandler
	}

	return mws(handler)(ch.Context(), ch, message)
}

// Release releases channel's specific hooks and reuse pipeline
func (pl *pipeline) Release() {
	pl.chocc = nil
	pl.chIn = nil
	pl.chOut = nil

	pool.Put(pl)
}

func emptyHandler(_ context.Context, _ less.Channel, _ interface{}) error { return nil }
