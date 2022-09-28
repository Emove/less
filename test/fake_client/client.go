package fake_client

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"runtime/debug"
)

type Option func(c *Client)

type Client struct {
	network   string
	addr      string
	ctx       context.Context
	cancel    context.CancelFunc
	conn      net.Conn
	onConnect func(conn net.Conn)
	onMessage func(conn net.Conn, msg []byte) error
}

func NewClient(network string, addr string, ops ...Option) *Client {
	client := &Client{network: network, addr: addr}
	client.ctx, client.cancel = context.WithCancel(context.Background())
	for _, op := range ops {
		op(client)
	}
	return client
}

func ConnectSuccess(fn func(conn net.Conn)) Option {
	return func(c *Client) {
		c.onConnect = fn
	}
}

func OnMessage(fn func(conn net.Conn, msg []byte) error) Option {
	return func(c *Client) {
		c.onMessage = fn
	}
}

func (c *Client) Dial() {
	dial, err := net.Dial(c.network, c.addr)
	if err != nil {
		panic(fmt.Sprintf("dial err: %v\n, stack: %s", err, string(debug.Stack())))
	}
	c.conn = dial
	if c.onConnect != nil {
		c.onConnect(dial)
	}
	go c.read()
}

func (c *Client) Close() {
	c.cancel()
	_ = c.conn.Close()
}

func (c *Client) read() {
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			header := make([]byte, binary.MaxVarintLen32)
			_, err := c.conn.Read(header)
			if err != nil {
				panic(fmt.Sprintf("read err: %v\n, stack: %s", err, string(debug.Stack())))
			}

			bodyLength := binary.BigEndian.Uint32(header)

			msg := make([]byte, bodyLength)
			_, err = c.conn.Read(msg)
			if err != nil {
				panic(fmt.Sprintf("read err: %v\n, stack: %s", err, string(debug.Stack())))
			}

			err = c.onMessage(c.conn, msg)
			if err != nil {
				return
			}
		}
	}
}
