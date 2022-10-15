package transport

import "context"

type GracefulCloser interface {
	Close(ctx context.Context, err error) error
}
