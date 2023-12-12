package design

import (
	"context"
	"net"
)

type Closer interface {
	close()
}

type Listener interface {
	Closer
	Listen() (Acceptor, error)
}

type Dialer interface {
	Dial(net, addr string) error
}

type Acceptor interface {
	Closer
	accept() (Connection, error)
}

type Connection interface {
	Read(buf []byte) (n int, err error)
	Reader() Reader
	Writer() Writer
	IsActive() bool
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

type EventDriver interface {
	OnConnect(ctx context.Context, con Connection) (context.Context, error)
	OnMessage(ctx context.Context, con Connection) error
	OnConnClosed(ctx context.Context, con Connection, err error)
}

type Channel interface {
	Context() context.Context
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	Write(msg interface{}) error
	IsActive() bool
	Close(err error)
	CloseReader()
	CloseWriter()
	Readable() bool
	Writeable() bool
}

type Handler func(ctx context.Context, ch Channel, message interface{}) error

type Middleware func(handler Handler) Handler

type Pipeline struct {
	inbound_wms []Middleware
	outound_wms []Middleware
}

type Codec interface {
	Encode(readerWriter ReadWriter, msg interface{}) error
	Decode(readerWriter ReadWriter) (interface{}, error)
}

type TransHandler interface {
	BoundHandler
	EventDriver
}

type BoundHandler interface {
	OnRead(reader Reader) (interface{}, error)
	OnWrite(writer Writer, msg interface{}) error
}

type Reader interface {
	Read(buff []byte) (n int, err error)
	Next(n int) (buf []byte, err error)
	Peek(n int) (buf []byte, err error)
	Skip(n int) (err error)
	Length() int
	Release()
}

type Writer interface {
	Write(buf []byte) (n int, err error)
	Malloc(n int) (buf []byte, err error)
	MallocLength() (length int)
	Flush() (err error)
	Release()
}

type ReadWriter interface {
	Reader
	Writer
}
