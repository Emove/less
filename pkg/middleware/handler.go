package middleware

import (
	"less/pkg/transport"
)

type Handler func(ctx transport.Context) error
