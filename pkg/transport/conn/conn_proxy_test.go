package conn

import (
	"context"
	"io"
	"less/pkg/proto"
	"less/pkg/transport"
	"log"
	"testing"
	"time"
)

func TestConProxy_Read(t *testing.T) {
	pair, err := prepare()
	if err != nil {
		log.Fatal(err.Error())
	}

	defer func() {
		_ = pair.client.Close()
		_ = pair.server.Close()
	}()

	ctx, cancelFunc := context.WithCancel(context.Background())
	cp := ConProxy{
		Ctx:       ctx,
		Raw:       pair.server,
		HeaderLen: proto.GenericProtocol.HeaderLength(),
		OnRequest: func(con transport.Connection) error {
			reader := con.Reader()
			header, err := reader.Next(int(proto.GenericProtocol.HeaderLength()))
			if err != nil {
				log.Fatal(err.Error())
				return err
			}
			bodySize := proto.GenericProtocol.BodySize(header)
			body, err := reader.Next(int(bodySize))
			if err != nil {
				log.Fatal(err.Error())
				return err
			}
			log.Printf("body: %s\n", string(body))
			return nil
		},
		OnConnClose: func(conn transport.Connection, err error) {
			_ = conn.Close()
		},
	}
	cp.Conn = WrapConnection(&cp)

	go cp.ReadLoop()

	body := "hello! This is client!"
	header := make([]byte, 4)
	bodySize := uint32(len(body))
	proto.GenericProtocol.ByteOrder().PutUint32(header, bodySize)
	content := append(header, []byte(body)...)
	for i := 0; i < 3; i++ {
		if _, err = pair.client.Write(content); err != nil {
			cancelFunc()
			log.Fatal(err.Error())
			return
		}
	}

	time.Sleep(1 * time.Second)
	cancelFunc()
}

func TestConProxy_Write(t *testing.T) {
	pair, err := prepare()
	if err != nil {
		log.Fatal(err.Error())
	}

	defer func() {
		_ = pair.client.Close()
		_ = pair.server.Close()
	}()

	ctx, cancelFunc := context.WithCancel(context.Background())
	cp := ConProxy{
		Ctx:       ctx,
		Raw:       pair.client,
		HeaderLen: proto.GenericProtocol.HeaderLength(),
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				header := make([]byte, proto.GenericProtocol.HeaderLength())
				if _, err := pair.server.Read(header); err != nil && err != io.EOF {
					log.Fatal("read Header err: " + err.Error())
					return
				}
				size := proto.GenericProtocol.BodySize(header)
				if size == 0 {
					continue
				}
				body := make([]byte, size)
				if _, err := io.ReadFull(pair.server, body); err != nil {
					log.Fatal("read body err: " + err.Error())
					return
				}
				log.Printf("body: %s\n", string(body))
			}
		}
	}()

	body := "hello! This is client!"
	header := make([]byte, 4)
	bodySize := uint32(len(body))
	proto.GenericProtocol.ByteOrder().PutUint32(header, bodySize)
	content := append(header, []byte(body)...)
	for i := 0; i < 3; i++ {
		if _, err = cp.Write(content); err != nil {
			cancelFunc()
			log.Fatal("cp write err: " + err.Error())
			return
		}
	}

	time.Sleep(1 * time.Second)
	cancelFunc()
}

func TestConProxy_Close(t *testing.T) {
	pair, err := prepare()
	if err != nil {
		log.Fatal(err.Error())
	}

	defer func() {
		//err := pair.client.Close()
		//if err != nil {
		//	log.Println(err.Error())
		//}
		_ = pair.server.Close()
	}()

	cp := ConProxy{
		Raw:       pair.client,
		HeaderLen: proto.GenericProtocol.HeaderLength(),
	}

	for i := 0; i < 2; i++ {
		if _, err = cp.Write([]byte("content")); err != nil {
			log.Println(err.Error())
			return
		}
		_ = cp.Close()
	}
}

func TestConProxy_LocalAddr(t *testing.T) {
	pair, err := prepare()
	if err != nil {
		log.Fatal(err.Error())
	}

	defer func() {
		_ = pair.client.Close()
		_ = pair.server.Close()
	}()

	cp := ConProxy{
		Raw: pair.client,
	}

	log.Println(cp.LocalAddr())
}

func TestConProxy_RemoteAddr(t *testing.T) {
	pair, err := prepare()
	if err != nil {
		log.Fatal(err.Error())
	}

	defer func() {
		_ = pair.client.Close()
		_ = pair.server.Close()
	}()

	cp := ConProxy{
		Raw: pair.client,
	}

	log.Println(cp.RemoteAddr())
}
