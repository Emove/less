//go:build windows
// +build windows

package transport

import (
	"net"
	"time"
)

func NewConnection(conn net.Conn) Connection {
	return &connection{
		conn: conn,
	}
}

type connection struct {
	conn net.Conn

	readTimeout time.Duration

	idleTimeout time.Duration
}

func (c *connection) Read(b []byte) (n int, err error) {
	return c.conn.Read(b)
}

func (c *connection) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *connection) Close() error {
	return c.conn.Close()
}

func (c *connection) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *connection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *connection) SetReadTimeout(t time.Duration) error {
	if t >= 0 {
		c.readTimeout = t
	}
	return nil
}

func (c *connection) SetIdleTimeout(t time.Duration) error {
	if t >= 0 {
		c.idleTimeout = t
	}
	return nil
}
