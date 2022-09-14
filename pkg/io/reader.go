package io

// Reader an abstraction to read data from transport.Connection.
//
// All read operations are implemented as blocking.
// The return value is guaranteed to meet the requirements or an error will be returned.
type Reader interface {

	// Read implements io.Reader
	Read(buff []byte) (n int, err error)

	// Next returns the next n bytes when the connection data is ready, and returns the original buffer.
	Next(n int) (buf []byte, err error)

	// Peek returns the next n bytes but not advancing the reader.
	Peek(n int) (buf []byte, err error)

	// Skip skips the next n bytes.
	Skip(n int) (err error)

	// Until reads until the first occurrence of delim in the connection.
	// Until returns an error when the size of line over than server.ServerOptions#MaxMessageSize or read timeout.
	//Until(delim byte) (line []byte, err error)

	Length() int

	// Release releases the memory space occupied by all read slices.
	Release()
}
