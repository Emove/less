package codec

import "io"

type Frame interface {
	Bytes() []byte
	Retain()
	Release()
}

type ReaderBuffer interface {
	Peek(n int) ([]byte, error)
	Next(n int) ([]byte, error)
	Skip(n int) error
	Slice(n int) (Frame, error)
	Len() int
	Release()
}

type WriterBuffer interface {
	Malloc(n int) ([]byte, error)
	WriteBinary(p []byte) error
	WriteFrame(f Frame) error
	Append(src WriterBuffer) error
	Len() int
	FlushTo(dst io.Writer) error
	Release()
}

type PacketCodec interface {
	Name() string
	Encode(dst WriterBuffer, payload Frame) error
	Decode(src ReaderBuffer) (payload Frame, err error)
}

type PayloadCodec interface {
	Name() string
	Marshal(message any) (Frame, error)
	Unmarshal(payload Frame) (any, error)
}
