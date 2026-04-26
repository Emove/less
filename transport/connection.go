package transport

import (
	"net"
)

const (
	Active = iota
	Inactive
)

// Connection defines an interface of underlying connection.
type Connection interface {

	//Context() context.Context

	// SetContext(ctx context.Context)

	Read(buf []byte) (n int, err error)
	Write(buf []byte) (n int, err error)

	// IsActive returns false only when Connection closed.
	IsActive() bool

	// Close closes the connection.
	Close() error

	// LocalAddr returns the local network address, same as net.Conn#LocalAddr.
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address, same as net.Conn#RemoteAddr.
	RemoteAddr() net.Addr

	// SetReadTimeout sets the timeout for future Read calls wait.
	// A zero value for timeout means Reader will not be timeout.
	//SetReadTimeout(t time.Duration) error
}
