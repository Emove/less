package server

import (
	"less/pkg/middleware"
	"less/pkg/transport"
)

type ServerOptions struct {
	TransSrvOps   *transport.TransServerOption
	MsgHandlerOps *middleware.HandlerOptions
}

var DefaultServerOptions = ServerOptions{
	TransSrvOps:   transport.DefaultTransSrvOptions,
	MsgHandlerOps: &middleware.HandlerOptions{},
}
