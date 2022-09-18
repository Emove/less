package conn

import (
	"github.com/emove/less/pkg/io"
	"net"
)

const (
	Active = iota
	Inactive
)

// Connection defines an interface of underlying connection.
type Connection interface {
	Read(buf []byte) (n int, err error)

	// Reader returns a Reader with buffer size limit.
	Reader() io.Reader

	// Writer returns a Writer.
	Writer() io.Writer

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
