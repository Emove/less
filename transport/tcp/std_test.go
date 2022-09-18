package tcp

import (
	"fmt"
	"log"
	"net"
	"testing"
	"time"
)

type connPair struct {
	client net.Conn
	server net.Conn
}

func prepare() (pair *connPair, err error) {
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

	pair = &connPair{}
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

type fn func(pair *connPair)

func do(f fn) {
	pair, err := prepare()
	if err != nil {
		log.Fatalln(err)
	}

	defer func() {
		if e := recover(); e != nil {
			if pair.client != nil {
				_ = pair.client.Close()
			}

			if pair.server != nil {
				_ = pair.server.Close()
			}
		}
	}()

	f(pair)

	if pair.client != nil {
		_ = pair.client.Close()
	}

	if pair.server != nil {
		_ = pair.server.Close()
	}
}

func Test_connection_Close(t *testing.T) {
	do(func(pair *connPair) {
		client := WrapConnection(pair.client)
		if err := client.Close(); err != nil {
			fmt.Println(err)
		}

		if client.IsActive() {
			t.Fatal("want: false, but: true")
		}

		if err := client.Close(); err != nil {
			t.Fatalf("%v", err)
		}
	})
}

func Test_connection_IsActive(t *testing.T) {
	do(func(pair *connPair) {

		client := WrapConnection(pair.client)

		t.Logf("before close, client active status: %v", client.IsActive())

		if err := client.Close(); err != nil {
			t.Fatal(err)
		}

		if client.IsActive() {
			t.Fatal("client active status, want: false, but: true")
		}
		t.Logf("after close, client active status: %v", client.IsActive())
	})
}

func Test_connection_LocalAddr(t *testing.T) {
	do(func(pair *connPair) {

		client := WrapConnection(pair.client)

		if pair.client.LocalAddr() != client.LocalAddr() {
			t.Fatalf("want: %s, but: %s", pair.client.LocalAddr(), client.LocalAddr())
		}
	})
}

func Test_connection_Read(t *testing.T) {
	do(func(pair *connPair) {
		content := []byte("hello server")

		server := WrapConnection(pair.server)

		go func() {
			if _, err := pair.client.Write(content); err != nil {
				t.Errorf("write msg err: %v", err)
			}
		}()

		msg := make([]byte, len(content))
		_, err := server.Read(msg)
		if err != nil {
			t.Fatalf("read msg err: %v", err)
		}
		t.Logf("read msg: %s", string(msg))
	})
}

func Test_connection_Reader(t *testing.T) {
	do(func(pair *connPair) {
		content := []byte("hello server")

		server := WrapConnection(pair.server)
		go func() {
			if _, err := pair.client.Write(content); err != nil {
				t.Errorf("write msg err: %v", err)
			}
		}()

		reader := server.Reader()
		msg, err := reader.Peek(len(content))
		if err != nil {
			t.Fatalf("read peek err: %v", err)
		}
		t.Logf("read msg: %s", string(msg))

		t.Log("skip 6 bytes")
		err = reader.Skip(6)
		if err != nil {
			t.Fatalf("skip err: %v", err)
		}

		msg, err = reader.Next(len(content) - 6)
		if err != nil {
			t.Fatalf("read next err: %v", err)
		}
		t.Logf("read next: %s", string(msg))
	})
}

func Test_connection_RemoteAddr(t *testing.T) {
	do(func(pair *connPair) {
		con := WrapConnection(pair.client)
		if pair.client.RemoteAddr() != con.RemoteAddr() {
			t.Fatalf("remote addr error")
		}
	})
}

func Test_connection_SetReadTimeout(t *testing.T) {
	// ignore
}

func Test_connection_Writer(t *testing.T) {
	do(func(pair *connPair) {
		client := WrapConnection(pair.client)

		content := []byte("hello server")

		go func() {
			writer := client.Writer()
			_, err := writer.Write(content[:6])
			if err != nil {
				t.Errorf("client write 'hello ' error: %s", err.Error())
				return
			}
			malloc := writer.Malloc(6)
			copy(malloc, content[6:])

			if writer.MallocLength() != len(content) {
				t.Errorf("write %d bytes to writer, but MallocLength got %d", len(content), writer.MallocLength())
				return
			}

			if err = writer.Flush(); err != nil {
				t.Errorf("writer flush error: %s", err.Error())
				return
			}
		}()

		buf := make([]byte, len(content))

		_, err := pair.server.Read(buf)
		if err != nil {
			t.Fatalf("server read msg error: %s", err.Error())
		}
		t.Logf("server read msg: %s", string(buf))
	})
}
