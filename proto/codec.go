package proto

import (
	"context"
	"less/transport"
)

type Codec interface {
	Encode(ctx context.Context, res interface{}, writer transport.Writer) (err error)
	Decode(ctx context.Context, reader transport.Reader) (req interface{}, err error)
}
