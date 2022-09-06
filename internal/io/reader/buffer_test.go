package reader

import (
	"log"
	"net"
	"reflect"
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

func TestNewBufferReader(t *testing.T) {
	do(func(pair *connPair) {
		reader := NewBufferReader(pair.server)
		br := reader.(*bufferReader)

		if len(br.buff) != 1024 {
			t.Errorf("expected reader buf length: 1024, but: %d", len(br.buff))
		}
	})
}

func TestNewBufferReaderWithBuf(t *testing.T) {
	do(func(pair *connPair) {
		size := 8
		buf := make([]byte, size)

		reader := NewBufferReaderWithBuf(pair.server, buf)

		br := reader.(*bufferReader)

		if len(br.buff) != size {
			t.Errorf("expected reader buf length: 8, but: %d", len(br.buff))
		}

		if _, err := br.Next(9); err != nil {
			t.Fatalf(err.Error())
		}

	})
}

func Test_bufferReader_Next(t *testing.T) {
	msg := []byte(("hello server!"))
	size := len(msg)
	do(func(pair *connPair) {
		buf := make([]byte, size)

		reader := NewBufferReaderWithBuf(pair.server, buf)

		if _, err := pair.client.Write(msg); err != nil {
			t.Fatalf("client write msg error, %s", err.Error())
		}

		// read "hello"
		next, err := reader.Next(5)
		if err != nil {
			t.Fatalf("next error, %s", err.Error())
		}
		t.Logf("read 5 bytes, want: 'hello', got: %s", string(next))

		// read " server!"
		remain, err := reader.Next(8)
		if err != nil {
			t.Fatalf("next error, '%s'", err.Error())
		}
		t.Logf("read 8 bytes, want: ' server!', got: '%s'", string(remain))

		if !reflect.DeepEqual(msg, append(next, remain...)) {
			t.Fatalf("read msg error, want: %s, got: %s", string(msg), string(next))
		}

	})
}

func Test_bufferReader_Peek(t *testing.T) {
	msg := []byte(("hello server!"))
	size := len(msg)
	do(func(pair *connPair) {
		buf := make([]byte, size)

		reader := NewBufferReaderWithBuf(pair.server, buf).(*bufferReader)

		if _, err := pair.client.Write(msg); err != nil {
			t.Fatalf("client write msg error, %s", err.Error())
		}

		peekTest := func(n int) {
			next, err := reader.Peek(n)
			if err != nil {
				t.Fatalf("peek error, %s", err.Error())
			}
			t.Logf("peek %d bytes, got: %s", n, string(next))

			if reader.readIndex != 0 {
				t.Fatalf("read index error, excepted: %d, but: %d", 0, reader.readIndex)
			}

			if reader.writeIndex != n {
				t.Fatalf("write index error, excepted: %d, but: %d", n, reader.writeIndex)
			}
		}

		// read "hello"
		peekTest(len("hello"))

		// read "hello server!"
		peekTest(size)

		next, err := reader.Next(size)
		if err != nil {
			t.Fatalf("next error, '%s'", err.Error())
		}
		if !reflect.DeepEqual(msg, next) {
			t.Fatalf("read msg error, want: %s, got: %s", string(msg), string(next))
		}

		go func() {
			// will be blocked
			peek, err := reader.Peek(1)
			if err == nil {
				t.Logf("peek more: %s", string(peek))
			}
		}()

		time.Sleep(time.Second)
		_, _ = pair.client.Write([]byte("1"))
	})
}

func Test_bufferReader_Release(t *testing.T) {
	msg := []byte(("hello server!"))
	size := len(msg)
	do(func(pair *connPair) {
		buf := make([]byte, size)

		reader := NewBufferReaderWithBuf(pair.server, buf).(*bufferReader)

		_, _ = pair.client.Write(msg)
		_, _ = reader.Next(size)

		reader.Release()
		if reader.decorator != nil ||
			reader.readIndex != 0 ||
			reader.writeIndex != 0 ||
			len(reader.buff) != 0 {
			t.Fatal()
		}
	})
}

func Test_bufferReader_Skip(t *testing.T) {
	msg := []byte(("hello server!"))
	size := len(msg)
	do(func(pair *connPair) {
		buf := make([]byte, size)

		reader := NewBufferReaderWithBuf(pair.server, buf).(*bufferReader)

		if _, err := pair.client.Write(msg); err != nil {
			t.Fatalf("client write msg error, %s", err.Error())
		}

		// skip "hello "
		hl := len("hello ")
		if err := reader.Skip(hl); err != nil {
			t.Fatalf("skip err: %s", err.Error())
		}
		if reader.readIndex != hl {
			t.Errorf("read index error, excepted: %d, but: %d", hl, reader.readIndex)
		}
		if reader.writeIndex != hl {
			t.Errorf("write index error, excepted: %d, but: %d", hl, reader.writeIndex)
		}

		// read "server"
		s := "server"
		peek, err := reader.Next(len(s))
		if err != nil {
			t.Fatalf("peek err: %s", err.Error())
		}

		if s != string(peek) {
			t.Fatalf("want: %s, got: %s", s, string(peek))
		}

		if err = reader.Skip(1); err != nil {
			t.Fatalf("skip err: %s", err.Error())
		}
		if reader.readIndex != size {
			t.Errorf("read index error, excepted: %d, but: %d", size, reader.readIndex)
		}
		if reader.writeIndex != size {
			t.Errorf("write index error, excepted: %d, but: %d", size, reader.writeIndex)
		}

	})
}
