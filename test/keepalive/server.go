package keepalive

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/emove/less/server"
	"net"
	"sync"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/keepalive"
	"github.com/emove/less/log"
	"github.com/emove/less/router"
	"github.com/emove/less/test/fake_client"
)

var ping = []byte("ping")
var pong = []byte("pong")
var goaway = []byte("go away")

func newServer(kp *keepalive.KeepaliveParameters) *server.Server {
	onChannelOption := server.WithOnChannel(ocIdentifier())
	onChannelClosedOption := server.WithOnChannelClosed(deleteOnChannelClosed())
	if kp != nil {
		return server.NewServer("localhost", onChannelOption, onChannelClosedOption, server.WithRouter(newRouter()), server.KeepaliveParams(*kp))
	}
	return server.NewServer("localhost", onChannelOption, onChannelClosedOption, server.WithRouter(newRouter()))
}

var wg = &sync.WaitGroup{}

func KeepaliveServer(kp *keepalive.KeepaliveParameters) {
	server := newServer(kp)

	server.Run()

	wg.Add(1)
	time.AfterFunc(20*time.Second, func() {
		wg.Done()
	})

	write := func(conn net.Conn, msg string) {
		header := make([]byte, binary.MaxVarintLen32)
		binary.BigEndian.PutUint32(header, uint32(len(msg)))
		content := append(header, []byte(msg)...)
		_, _ = conn.Write(content)
	}

	var c *fake_client.Client
	var cc net.Conn
	c = fake_client.NewClient("tcp", "localhost:8888", fake_client.ConnectSuccess(func(conn net.Conn) {
		cc = conn
	}), fake_client.OnMessage(func(conn net.Conn, msg []byte) error {

		m := string(msg)
		log.Infof("client receive: %s", m)
		if m == "ping" {
			time.Sleep(2 * time.Second)
			write(conn, "pong")
		}

		if m == "go away" {
			c.Close()
		}

		return nil
	}))

	c.Dial()

	go func() {
		for i := 0; i < 2; i++ {
			write(cc, fmt.Sprintf("client msg%d", i))
			time.Sleep(4 * time.Second)
		}
	}()

	wg.Wait()
	server.Shutdown()
}

var IDGenerator uint32

type ctxIdentifierKey struct{}

type IdentifierChannel struct {
	id uint32
	ch less.Channel
}

func ocIdentifier() less.OnChannel {
	return func(ctx context.Context, ch less.Channel) (context.Context, error) {
		IDGenerator++
		ich := &IdentifierChannel{id: IDGenerator, ch: ch}
		channels[IDGenerator] = ich
		return context.WithValue(ctx, ctxIdentifierKey{}, ich), nil
	}
}

var channels = make(map[uint32]*IdentifierChannel)

func deleteOnChannelClosed() less.OnChannelClosed {
	return func(ctx context.Context, ch less.Channel, err error) {
		if ich := ctx.Value(ctxIdentifierKey{}); ich != nil {
			if c, ok := ich.(*IdentifierChannel); ok {
				log.Infof("channel closed, id: %d, err: %v\n", c.id, err)
				wg.Done()
			}
		}
	}
}

func newRouter() router.Router {
	once := sync.Once{}
	return func(ctx context.Context, channel less.Channel, msg interface{}) (less.Handler, error) {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			ich := ctx.Value(ctxIdentifierKey{}).(*IdentifierChannel)
			log.Infof("channel id: %d, message: %v", ich.id, message)

			// try to close channel
			//_ = channel.Close(context.Background(), nil)

			once.Do(func() {
				_ = ch.Write("hi client!")
			})
			return nil
		}, nil
	}
}
