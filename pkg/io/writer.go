package io

// Writer is an abstraction to write data to transport.Connection.
type Writer interface {

	// Write writes bytes to buffer directly.
	Write(buf []byte) (n int, err error)

	// Malloc returns the next n bytes slice from the buffer.
	Malloc(n int) (buf []byte, err error)

	// MallocLength returns the total length of the written data that has not yet been submitted in the writer.
	MallocLength() (length int)

	// Flush submits all written data to raw connection.
	Flush() (err error)

	// Release the memory space occupied by all write slices.
	Release()
}
