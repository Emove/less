package conn

import (
	"less/pkg/transport"
	"net"
	"testing"
	"time"
)

func TestConnection_Reader(t *testing.T) {
	pair, err := prepare()
	if err != nil {
		t.Fatalf("prepare tcp connection error: %s", err.Error())
	}
	defer func() {
		// close tcp conn
		//_ = pair.conn.Close()
		_ = pair.client.Close()
	}()

	serverConn := wrapConnection(pair.server)
	defer func(serverConn transport.Connection) {
		_ = serverConn.Close()
	}(serverConn)

	_, err = pair.client.Write([]byte("hello! I'm client"))
	if err != nil {
		t.Fatalf("client write data error: %s", err.Error())
	}

	reader := serverConn.Reader()
	firstSixByte, err := reader.Next(6)
	if err != nil {
		t.Fatalf("conn connection reader occur error when read next n byte: %s", err.Error())
	}
	// expected output "hello!"
	t.Logf("first 6 byte is: %s", string(firstSixByte))

	if err = reader.Skip(1); err != nil {
		// skip a space
		t.Fatalf("conn connection reader occur error when read skip n byte: %s", err.Error())
	}

	peek, err := reader.Peek(3)
	if err != nil {
		t.Fatalf("conn connection reader occur error when read peek n byte: %s", err.Error())
	}
	// expected output "I'm"
	t.Logf("peek 3 byte is: %s", string(peek))

	until, err := reader.Until('t')
	if err != nil {
		t.Fatalf("conn connection reader occur error when read until 't': %s", err.Error())
	}
	// expected output "I'm client"
	t.Logf("until 't' is: %s", string(until))
}

type ConnPair struct {
	client net.Conn
	server net.Conn
}

func prepare() (pair *ConnPair, err error) {
	network, addr := "tcp", ":8080"
	listen, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}

	type clientCon struct {
		client net.Conn
		err    error
	}

	clientChann := make(chan *clientCon)
	go func() {
		time.Sleep(1 * time.Second)
		client, err1 := net.Dial(network, addr)
		clientChann <- &clientCon{client: client, err: err1}
	}()

	pair = &ConnPair{}
	defer func() {
		if err != nil {
			if pair.server != nil {
				_ = pair.server.Close()
			}
			if pair.client != nil {
				_ = pair.client.Close()
			}
		}
	}()

	for {
		var con net.Conn
		con, err = listen.Accept()
		if err != nil {
			return
		}
		pair.server = con
		break
	}

	select {
	case cc := <-clientChann:
		if cc.err != nil {
			return nil, err
		}
		pair.client = cc.client
		return
	}
}
