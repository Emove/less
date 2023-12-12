package server

import (
	"context"
	"encoding/binary"
	"errors"
	"github.com/emove/less"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/emove/less/log"
	"github.com/emove/less/router"
)

func newServer() *Server {
	onChannelOption := WithOnChannel(ocAddressChecker(), ocIdentifier())
	onChannelClosedOption := WithOnChannelClosed(deleteOnChannelClosed())
	inboundOption := WithInboundMiddleware(newInboundMiddleware())
	outboundOption := WithOutboundMiddleware(newOutboundMiddleware())
	return NewServer("localhost", onChannelOption, onChannelClosedOption,
		inboundOption, outboundOption, WithRouter(newRouter()),
		//DisableGoPool(),
	)
}

var wg = &sync.WaitGroup{}

func TestServer_Run(t *testing.T) {
	server := newServer()

	server.Run()

	wg.Add(1)
	go func() {
		mockClient(t)
	}()

	wg.Wait()
	server.Shutdown(context.Background(), nil)
}

func mockClient(t *testing.T) {
	con, err := net.Dial("tcp", "localhost:8888")
	if err != nil {
		t.Fatalf("client dial err: %v\n", err)
	}

	msg := []byte("hello server!")
	header := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(header, uint32(len(msg)))

	packet := append(header, msg...)

	_, err = con.Write(packet)
	if err != nil {
		t.Fatalf("client write msg err: %v\n", err)
	}

	header = make([]byte, binary.MaxVarintLen32)

	if _, err = con.Read(header); err != nil {
		t.Fatalf("client read msg header err: %v\n", err)
	}

	length := binary.BigEndian.Uint32(header)
	body := make([]byte, length, length)

	if _, err = con.Read(body); err != nil {
		t.Fatalf("client read msg body err: %v\n", err)
	}

	log.Infof("client read msg: %s", string(body))

	msg = []byte("i will close connection after 1 sec")
	header = make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(header, uint32(len(msg)))
	_, _ = con.Write(append(header, msg...))

	time.Sleep(time.Second)
	_ = con.Close()
}

func ocAddressChecker() less.OnChannel {
	return func(ctx context.Context, ch less.Channel) (context.Context, error) {
		addr, _, err := net.SplitHostPort(ch.RemoteAddr().String())
		if err != nil {
			return ctx, err
		}
		if ch.RemoteAddr() != nil && addr != "127.0.0.1" {
			log.Errorf("refused a connection from: %s", addr)
			return nil, errors.New("allows 127.0.0.1 address ")
		}
		log.Infof("receive a connection from: %s, network: %s", ch.RemoteAddr().String(), ch.RemoteAddr().Network())
		return ctx, nil
	}
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

func newInboundMiddleware() less.Middleware {
	return func(handler less.Handler) less.Handler {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			log.Infof("inbound before")
			err := handler(ctx, ch, message)
			log.Infof("inbound after")
			return err
		}
	}
}

func newOutboundMiddleware() less.Middleware {
	return func(handler less.Handler) less.Handler {
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			log.Infof("outbound before")
			err := handler(ctx, ch, message)
			log.Infof("outbound after")
			return err
		}
	}
}

func newRouter() router.Router {
	return func(ctx context.Context, channel less.Channel, msg interface{}) (less.Handler, error) {
		once := sync.Once{}
		return func(ctx context.Context, ch less.Channel, message interface{}) error {
			ich := ctx.Value(ctxIdentifierKey{}).(*IdentifierChannel)
			log.Infof("channel id: %d, message: %v", ich.id, message)
			once.Do(func() {
				_ = ch.Write("hi client!")
			})
			return nil
		}, nil
	}
}
