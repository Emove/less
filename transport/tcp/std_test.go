package tcp

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"testing"
	"time"

	trans "github.com/emove/less/transport"
)

type connPair struct {
	client net.Conn
	server net.Conn
}

func TestTransport_ImplementsUnifiedTransport(t *testing.T) {
	var _ trans.Transport = New()
}

func prepare() (pair *connPair, err error) {
	network, addr := "tcp", "127.0.0.1:0"
	listen, err := net.Listen(network, addr)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = listen.Close()
	}()

	addr = listen.Addr().String()

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

type noopEventDriver struct {
	onConnect    func(context.Context, trans.Connection) (context.Context, error)
	onMessage    func(context.Context, trans.Connection) error
	onConnClosed func(context.Context, trans.Connection, error)
}

func (d noopEventDriver) OnConnect(ctx context.Context, conn trans.Connection) (context.Context, error) {
	if d.onConnect != nil {
		return d.onConnect(ctx, conn)
	}
	return ctx, nil
}

func (d noopEventDriver) OnMessage(ctx context.Context, conn trans.Connection) error {
	if d.onMessage != nil {
		return d.onMessage(ctx, conn)
	}
	time.Sleep(10 * time.Millisecond)
	return nil
}

func (d noopEventDriver) OnConnClosed(ctx context.Context, conn trans.Connection, err error) {
	if d.onConnClosed != nil {
		d.onConnClosed(ctx, conn, err)
	}
}

func reserveTCPAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve tcp addr: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	return listener.Addr().String()
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

func Test_connection_Write(t *testing.T) {
	do(func(pair *connPair) {
		client := WrapConnection(pair.client)
		method := reflect.ValueOf(client).MethodByName("Write")
		if !method.IsValid() {
			t.Fatalf("WrapConnection() does not expose Write([]byte)")
		}
		if method.Type().NumIn() != 1 {
			t.Fatalf("Write() got %d inputs, want 1", method.Type().NumIn())
		}
		if method.Type().NumOut() != 1 && method.Type().NumOut() != 2 {
			t.Fatalf("Write() got %d outputs, want 1 or 2", method.Type().NumOut())
		}
		errType := method.Type().Out(method.Type().NumOut() - 1)
		if !errType.Implements(reflect.TypeOf((*error)(nil)).Elem()) {
			t.Fatalf("Write() final output = %v, want error", errType)
		}

		content := []byte("hello server")
		argType := method.Type().In(0)
		if !reflect.TypeOf(content).AssignableTo(argType) {
			t.Fatalf("Write() parameter type = %v, want []byte-compatible input", argType)
		}

		writeDone := make(chan []reflect.Value, 1)
		go func() {
			writeDone <- method.Call([]reflect.Value{reflect.ValueOf(content)})
		}()

		buf := make([]byte, len(content))
		if err := pair.server.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			t.Fatalf("set read deadline: %v", err)
		}
		if _, err := pair.server.Read(buf); err != nil {
			t.Fatalf("server read msg error: %v", err)
		}
		if err := pair.server.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline: %v", err)
		}

		select {
		case results := <-writeDone:
			gotErr := results[len(results)-1].Interface()
			if gotErr != nil {
				err, ok := gotErr.(error)
				if !ok {
					t.Fatalf("Write() final result has dynamic type %T, want error", gotErr)
				}
				t.Fatalf("Write() error = %v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("Write() call did not complete")
		}

		t.Logf("server read msg: %s", string(buf))
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

func TestConnectionInterface_DoesNotExposeBufferedIO(t *testing.T) {
	connType := reflect.TypeOf((*trans.Connection)(nil)).Elem()

	if _, ok := connType.MethodByName("Reader"); ok {
		t.Errorf("transport.Connection exposes Reader(), want Read() only")
	}
	if _, ok := connType.MethodByName("Writer"); ok {
		t.Errorf("transport.Connection exposes Writer(), want Write() only")
	}
	if _, ok := connType.MethodByName("Read"); !ok {
		t.Errorf("transport.Connection does not expose Read()")
	}
	if _, ok := connType.MethodByName("Write"); !ok {
		t.Errorf("transport.Connection does not expose Write()")
	}
}

func Test_transport_Close_StopsAcceptAndRejectsNewDials(t *testing.T) {
	addr := reserveTCPAddr(t)
	tr := New().(*transport)

	listenErrCh := make(chan error, 1)
	go func() {
		listenErrCh <- tr.Listen(addr, noopEventDriver{})
	}()

	deadline := time.After(time.Second)
	for {
		conn, err := net.DialTimeout("tcp", addr, 20*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}

		select {
		case <-deadline:
			t.Fatalf("transport did not start listening on %s: %v", addr, err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	tr.Close()

	select {
	case err := <-listenErrCh:
		if err != nil {
			t.Fatalf("Listen returned unexpected error after Close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Transport.Close() did not stop the accept loop")
	}

	conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected dial to fail after Transport.Close(), but it succeeded")
	}
}

func TestNew_AppliesTCPOptions(t *testing.T) {
	tr := New(
		WithNetwork(TCP4),
		WithTimeout(2*time.Second),
		WithKeepalive(false),
		WithKeepalivePeriod(3*time.Second),
		WithLinger(7),
		WithNoDelay(false),
	).(*transport)

	if tr.ops.Network != TCP4 {
		t.Fatalf("Network = %q, want %q", tr.ops.Network, TCP4)
	}
	if tr.ops.Timeout != 2*time.Second {
		t.Fatalf("Timeout = %v, want %v", tr.ops.Timeout, 2*time.Second)
	}
	if tr.ops.Keepalive {
		t.Fatal("Keepalive = true, want false")
	}
	if tr.ops.KeepAlivePeriod != 3*time.Second {
		t.Fatalf("KeepAlivePeriod = %v, want %v", tr.ops.KeepAlivePeriod, 3*time.Second)
	}
	if tr.ops.Linger != 7 {
		t.Fatalf("Linger = %d, want %d", tr.ops.Linger, 7)
	}
	if tr.ops.NoDelay {
		t.Fatal("NoDelay = true, want false")
	}
}

func TestNew_ClonesDefaultOptionsPerTransport(t *testing.T) {
	first := New(WithTimeout(0)).(*transport)
	second := New().(*transport)

	if first.ops == second.ops {
		t.Fatal("expected New to clone TCP default options per transport")
	}

	if second.ops.Timeout != DefaultOptions.Timeout {
		t.Fatalf("second transport Timeout = %v, want default %v", second.ops.Timeout, DefaultOptions.Timeout)
	}
}
