package transport

type GracefulCloser interface {
	Close() error
}
