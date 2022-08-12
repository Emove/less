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

	serverConn := WrapConnection(pair.server)
	defer func(serverConn transport.Connection) {
		_ = serverConn.Close()
	}(serverConn)

	content := []byte("hello! I'm client!")
	_, err = pair.client.Write(content)
	if err != nil {
		t.Fatalf("client write data error: %s", err.Error())
	}

	reader := serverConn.Reader()
	firstSixByte, err := reader.Next(6)
	if err != nil {
		t.Fatalf("connection reader error occur when read next n byte: %s", err.Error())
	}
	t.Logf("first 6 byte, want: hello!, got: %s", string(firstSixByte))

	if err = reader.Skip(1); err != nil {
		// skip a space
		t.Fatalf("connection reader error occur when read skip n byte: %s", err.Error())
	}

	peek, err := reader.Peek(3)
	if err != nil {
		t.Fatalf("connection reader error occur when read peek n byte: %s", err.Error())
	}
	t.Logf("peek 3 byte, want: I'm, got: %s", string(peek))

	//until, err := reader.Until('t')
	//if err != nil {
	//	t.Fatalf("connection reader error occur when read until 't': %s", err.Error())
	//}
	//t.Logf("until 't', want: I'm client, got: %s", string(until))

	last, err := reader.Next(11)
	if err != nil {
		t.Fatalf("connection reader error occur when read last byte: %s", err.Error())
	}
	t.Logf("next all, want: I'm client!, got: %s", string(last))
}

func TestConnection_Writer(t *testing.T) {
	pair, err := prepare()
	if err != nil {
		t.Fatalf("prepare tcp connection error: %s", err.Error())
	}
	defer func() {
		// close tcp conn
		_ = pair.client.Close()
	}()

	serverConn := WrapConnection(pair.server)
	defer func(serverConn transport.Connection) {
		_ = serverConn.Close()
	}(serverConn)

	writer := serverConn.Writer()
	_, err = writer.Write([]byte("Hi! "))
	if err != nil {
		t.Fatalf("connection writer error occur when write bytes: %s", err.Error())
	}
	malloc := writer.Malloc(5)
	malloc[0] = 'T'
	malloc[1] = 'h'
	malloc[2] = 'i'
	malloc[3] = 's'
	malloc[4] = ' '

	_, err = writer.Write([]byte("server!"))
	if err != nil {
		t.Fatalf("connection writer error occur when write bytes: %s", err.Error())
	}

	total := writer.MallocLength()
	t.Logf("total write %d bytes", total)

	err = writer.Flush()
	if err != nil {
		t.Fatalf("connection writer error occur when flush: %s", err.Error())
	}

	clientReadBuf := make([]byte, total)
	_, err = pair.client.Read(clientReadBuf)
	if err != nil {
		t.Fatalf("connection writer error occur when flush: %s", err.Error())
	}
	t.Logf("client read: %s", string(clientReadBuf))

	more := make([]byte, 1)
	_ = pair.client.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
	_, err = pair.client.Read(more)
	if err != nil {
		t.Logf("there have no more bytes")
		return
	}
	t.Errorf("read more byte: %s", string(more))
}

func TestConnection_Writer_FlushErr(t *testing.T) {
	pair, err := prepare()
	if err != nil {
		t.Fatalf("prepare tcp connection error: %s", err.Error())
	}
	defer func() {
		_ = pair.client.Close()
	}()

	serverConn := WrapConnection(pair.server)
	//defer func(serverConn transport.Connection) {
	//	_ = serverConn.Close()
	//}(serverConn)

	writer := serverConn.Writer()
	_, err = writer.Write([]byte("test"))
	if err != nil {
		t.Fatalf("connection writer error occur when write bytes: %s", err.Error())
	}

	_ = pair.server.Close()

	err = writer.Flush()
	if err != nil {
		t.Logf("connection writer error occur when flush: %s", err.Error())
	}
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
