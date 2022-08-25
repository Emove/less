package io

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
