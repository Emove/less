package transport

import (
	"net"
	"time"
)

// Reader an abstraction to read data from transport.Connection.
//
// All read operations are implemented as blocking.
// The return value is guaranteed to meet the requirements or an error will be returned.
type Reader interface {

	// Next returns the next n bytes when the connection data is ready, and returns the original buffer.
	Next(n int) (buf []byte, err error)

	// Peek returns the next n bytes but not advancing the reader.
	Peek(n int) (buf []byte, err error)

	// Skip skips the next n bytes.
	Skip(n int) (err error)

	// Until reads until the first occurrence of delim in the connection.
	// Until returns an error when the size of line over than server.ServerOptions#MaxMessageSize or read timeout.
	Until(delim byte) (line []byte, err error)

	// Release releases the memory space occupied by all read slices.
	Release()
}

// Writer an abstraction to write data to transport.Connection.
type Writer interface {

	// Write writes bytes to buffer directly.
	Write(buf []byte) (n int, err error)

	// Malloc returns a slice containing the next n bytes from the buffer.
	Malloc(n int) (buf []byte)

	// MallocLength returns the total length of the written data that has not yet been submitted in the writer.
	MallocLength() (length int)

	// Flush will submit all written data to raw connection.
	Flush() (err error)

	// Release the memory space occupied by all write slices.
	Release()
}

// Connection wrapper for net.Conn and netpoll.Connection.
type Connection interface {
	// Reader returns a Reader.
	Reader() Reader

	// Writer returns a Writer.
	Writer() Writer

	// Close closes the connection.
	Close() error

	// LocalAddr returns the local network address, same as net.Conn#LocalAddr.
	LocalAddr() net.Addr

	// RemoteAddr returns the remote network address, same as net.Conn#RemoteAddr.
	RemoteAddr() net.Addr

	// SetReadTimeout sets the timeout for future Read calls wait.
	// A zero value for timeout means Reader will not be timeout.
	SetReadTimeout(t time.Duration) error

	// SetIdleTimeout sets the idle timeout of connections.
	SetIdleTimeout(t time.Duration) error
}
