package conn

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"less/pkg/transport"
)

type ConProxy struct {
	Ctx         context.Context
	Raw         net.Conn
	Header      []byte
	HeaderLen   uint32
	Conn        transport.Connection
	OnRequest   func(conn transport.Connection) error
	OnConnClose func(conn transport.Connection, err error)
}

func (c *ConProxy) Read(buf []byte) (n int, err error) {
	b := buf
	if c.Header != nil {
		copy(buf[:c.HeaderLen], c.Header)
		b = buf[c.HeaderLen:]
		c.Header = nil
	}
	return c.Raw.Read(b)
}

func (c *ConProxy) Write(buf []byte) (n int, err error) {
	return c.Raw.Write(buf)
}

func (c *ConProxy) Close() error {
	return c.Raw.Close()
}

func (c *ConProxy) LocalAddr() net.Addr {
	return c.Raw.LocalAddr()
}

func (c *ConProxy) RemoteAddr() net.Addr {
	return c.Raw.RemoteAddr()
}

func (c *ConProxy) SetDeadline(t time.Time) error {
	return c.Raw.SetDeadline(t)
}

func (c *ConProxy) SetReadDeadline(t time.Time) error {
	return c.Raw.SetReadDeadline(t)
}

func (c *ConProxy) SetWriteDeadline(t time.Time) error {
	return c.Raw.SetWriteDeadline(t)
}

func (c *ConProxy) ReadLoop() {
	for {
		select {
		case <-c.Ctx.Done():
			return
		default:
			header := make([]byte, c.HeaderLen)
			_, err := io.ReadFull(c, header)
			if err != nil {
				c.close(err)
				return
			}
			c.Header = header

			// trigger OnRequest Event
			if c.OnRequest == nil {
				c.close(fmt.Errorf("Conn's onRequst func can not be nil"))
				return
			}
			if err = c.OnRequest(c.Conn); err != nil {
				c.close(err)
				return
			}
		}
	}
}

func (c *ConProxy) close(err error) {
	if c.OnConnClose != nil {
		c.OnConnClose(c.Conn, err)
	} else {
		// TODO log err
	}
}
