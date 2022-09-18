package channel

import (
	"context"
	"sync"

	"github.com/emove/less"
)

type PipelineFactory func(ch *Channel) *pipeline

var pool = sync.Pool{}

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

func (pl *pipeline) AddOnChannelClosed(onChannelClosed ...less.OnChannelClosed) {
	if len(onChannelClosed) > 0 {
		pl.chocc = append(pl.chocc, onChannelClosed...)
	}
}

func (pl *pipeline) AddInbound(inbound ...less.Middleware) {
	if len(inbound) > 0 {
		pl.chIn = append(pl.chIn, inbound...)
	}
}

func (pl *pipeline) AddOutbound(outbound ...less.Middleware) {
	if len(outbound) > 0 {
		pl.chOut = append(pl.chOut, outbound...)
	}
}

func (pl *pipeline) FireOnChannel(ctx context.Context) (err error) {
	for _, onChannel := range pl.onChannelChain {
		ctx, err = onChannel(ctx, pl.ch)
		if err != nil {
			return err
		}
		pl.ch.SetContext(ctx)
	}
	pl.ch.active()
	return nil
}

func (pl *pipeline) FireOnChannelClosed(err error) {
	onChannelClosedChain := append(pl.onChannelClosedChain, pl.chocc...)
	for _, onChannelClosed := range onChannelClosedChain {
		onChannelClosed(pl.ch.Context(), pl.ch, err)
	}
}

func (pl *pipeline) FireInbound(message interface{}) error {

	pl.ch.inboundTasks.Add(1)
	defer pl.ch.inboundTasks.Done()

	mws := less.Chain(less.Chain(pl.inbound...), less.Chain(pl.chIn...))

	if pl.router != nil {
		mws = less.Chain(mws, pl.router)
	}

	return mws(emptyHandler)(pl.ch.Context(), pl.ch, message)
}

func (pl *pipeline) FireOutbound(message interface{}) error {
	mws := less.Chain(less.Chain(pl.chOut...), less.Chain(pl.outbound...))
	if pl.outboundHandler != nil {
		return mws(pl.outboundHandler)(pl.ch.Context(), pl.ch, message)
	}
	return mws(emptyHandler)(pl.ch.Context(), pl.ch, message)
}

func (pl *pipeline) Release() {
	pl.ch = nil
	pl.chocc = nil
	pl.chIn = nil
	pl.chOut = nil

	pool.Put(pl)
}

func emptyHandler(_ context.Context, _ less.Channel, _ interface{}) error { return nil }
