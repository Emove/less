package handler

import (
	"less/pkg/middleware"
	"less/pkg/transport"
)

type pipeline []middleware.Handler

func (pl pipeline) execute(ctx transport.Context) error {
	for _, handler := range pl {
		if err := handler(ctx); err != nil {
			return err
		}
	}
	return nil
}

type PipelineHandler struct {
	inboundPipeline pipeline

	outboundPipeline pipeline
}

func (h *PipelineHandler) Handle(ctx transport.Context) error {
	if ctx.Message().MessageType() == transport.MessageTypeInbound {
		return h.inboundPipeline.execute(ctx)
	} else {
		return h.outboundPipeline.execute(ctx)
	}
}

func NewPipelineHandler(msgHandlerOps *middleware.HandlerOptions) PipelineHandler {
	return PipelineHandler{
		inboundPipeline:  msgHandlerOps.InboundHandlers,
		outboundPipeline: msgHandlerOps.OutboundHandlers,
	}
}
