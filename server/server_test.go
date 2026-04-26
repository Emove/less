package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	stdio "io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	"github.com/emove/less/internal/engine/framebuf"

	"github.com/emove/less/log"
	"github.com/emove/less/transport"
)

func newServer(addr string) *Server {
	onChannelOption := WithOnChannel(ocAddressChecker(), ocIdentifier())
	onChannelClosedOption := WithOnChannelClosed(deleteOnChannelClosed())
	inboundOption := WithInboundMiddleware(newInboundMiddleware())
	outboundOption := WithOutboundMiddleware(newOutboundMiddleware())
	return NewServer(addr, onChannelOption, onChannelClosedOption,
		inboundOption, outboundOption, WithRouter(newRouter()),
		//DisableGoPool(),
	)
}

var wg = &sync.WaitGroup{}

func TestServer_Run(t *testing.T) {
	addr := reserveTCPAddr(t)
	server := newServer(addr)

	server.Run()

	wg.Add(1)
	go func() {
		mockClient(t, addr)
	}()

	wg.Wait()
	server.Shutdown(context.Background(), nil)
}

func TestServer_CodecOptionsAreAccepted(t *testing.T) {
	transport := newBlockingTransport()
	var captured less.Channel
	var inboundMessage string
	packetCodec := stubServerPacketCodec{}
	payloadCodec := stubServerPayloadCodec{}

	srv := NewServer(
		"127.0.0.1:18888",
		WithTransport(transport),
		WithPacketCodec(packetCodec),
		WithPayloadCodec(payloadCodec),
		WithOnChannel(func(ctx context.Context, ch less.Channel) (context.Context, error) {
			captured = ch
			return ctx, nil
		}),
		WithRouter(func(ctx context.Context, ch less.Channel, msg interface{}) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, message interface{}) error {
				inboundMessage = message.(string)
				return nil
			}, nil
		}),
	)

	srv.Run()

	select {
	case <-transport.started:
	case <-time.After(time.Second):
		t.Fatal("server did not start transport listener")
	}

	conn := &captureConn{}
	ctx, err := transport.driver.OnConnect(context.Background(), conn)
	if err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}
	if captured == nil {
		t.Fatal("expected OnChannel to capture an active channel")
	}

	if err := captured.Write("hello"); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	wantOutbound := append(make([]byte, 4), []byte("msg:hello")...)
	binary.BigEndian.PutUint32(wantOutbound[:4], uint32(len("msg:hello")))
	if got := conn.Bytes(); !bytes.Equal(got, wantOutbound) {
		t.Fatalf("connection bytes = %v, want %v", got, wantOutbound)
	}

	wantInbound := append(make([]byte, 4), []byte("msg:world")...)
	binary.BigEndian.PutUint32(wantInbound[:4], uint32(len("msg:world")))
	conn.readData = wantInbound
	if err := transport.driver.OnMessage(ctx, conn); err != nil {
		t.Fatalf("OnMessage failed: %v", err)
	}
	if got := inboundMessage; got != "world" {
		t.Fatalf("inbound message = %q, want %q", got, "world")
	}

	srv.Shutdown(context.Background(), nil)
}

func TestServer_Run_UsesGenericEndpointHandler(t *testing.T) {
	blocking := newBlockingTransport()
	received := make(chan string, 1)
	srv := NewServer(
		"127.0.0.1:18888",
		WithTransport(blocking),
		WithPacketCodec(stubServerPacketCodec{}),
		WithPayloadCodec(stubServerPayloadCodec{}),
		WithRouter(func(ctx context.Context, ch less.Channel, msg interface{}) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, message interface{}) error {
				select {
				case received <- message.(string):
				default:
				}
				return nil
			}, nil
		}),
	)

	srv.Run()

	select {
	case <-blocking.started:
	case <-time.After(time.Second):
		t.Fatal("server did not start transport listener")
	}

	inbound := append(make([]byte, 4), []byte("msg:through-run")...)
	binary.BigEndian.PutUint32(inbound[:4], uint32(len("msg:through-run")))
	conn := &captureConn{readData: inbound}

	ctx, err := blocking.driver.OnConnect(context.Background(), conn)
	if err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}
	if err := blocking.driver.OnMessage(ctx, conn); err != nil {
		t.Fatalf("OnMessage failed: %v", err)
	}

	select {
	case got := <-received:
		if got != "through-run" {
			t.Fatalf("router received %q, want %q", got, "through-run")
		}
	case <-time.After(time.Second):
		t.Fatal("expected router to receive the decoded message through Server.Run")
	}

	srv.Shutdown(context.Background(), nil)
}

type stubServerPacketCodec struct{}

func (stubServerPacketCodec) Name() string { return "stub-server-packet" }
func (stubServerPacketCodec) Encode(dst codec.WriterBuffer, payload codec.Frame) error {
	header, err := dst.Malloc(4)
	if err != nil {
		return err
	}
	binary.BigEndian.PutUint32(header, uint32(len(payload.Bytes())))
	return dst.WriteFrame(payload)
}
func (stubServerPacketCodec) Decode(src codec.ReaderBuffer) (codec.Frame, error) {
	header, err := src.Next(4)
	if err != nil {
		return nil, err
	}
	return src.Slice(int(binary.BigEndian.Uint32(header)))
}

type stubServerPayloadCodec struct{}

func (stubServerPayloadCodec) Name() string { return "stub-server-payload" }
func (stubServerPayloadCodec) Marshal(message any) (codec.Frame, error) {
	return framebuf.NewFrame([]byte("msg:" + message.(string))), nil
}
func (stubServerPayloadCodec) Unmarshal(payload codec.Frame) (any, error) {
	body := payload.Bytes()
	if !bytes.HasPrefix(body, []byte("msg:")) {
		return nil, errors.New("invalid payload")
	}
	return string(body[len("msg:"):]), nil
}

type captureConn struct {
	bytes.Buffer
	readData []byte
	closed   int32
}

func (c *captureConn) Read(buf []byte) (int, error) {
	if len(c.readData) == 0 {
		return 0, stdio.EOF
	}
	n := copy(buf, c.readData)
	c.readData = c.readData[n:]
	return n, nil
}
func (c *captureConn) Write(buf []byte) (int, error) { return c.Buffer.Write(buf) }
func (c *captureConn) IsActive() bool                { return atomic.LoadInt32(&c.closed) == 0 }
func (c *captureConn) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	return nil
}
func (c *captureConn) LocalAddr() net.Addr  { return shutdownAddr{} }
func (c *captureConn) RemoteAddr() net.Addr { return shutdownAddr{} }

func TestServer_CodecOptionsRejectTypedNil(t *testing.T) {
	t.Run("packet", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected WithPacketCodec to panic on typed nil")
			}
		}()
		var c *typedNilPacketCodec
		_ = WithPacketCodec(c)
	})

	t.Run("payload", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("expected WithPayloadCodec to panic on typed nil")
			}
		}()
		var c *typedNilPayloadCodec
		_ = WithPayloadCodec(c)
	})
}

type typedNilPacketCodec struct{}

func (*typedNilPacketCodec) Name() string { return "typed-nil-packet" }
func (*typedNilPacketCodec) Encode(codec.WriterBuffer, codec.Frame) error {
	return nil
}
func (*typedNilPacketCodec) Decode(codec.ReaderBuffer) (codec.Frame, error) {
	return nil, nil
}

type typedNilPayloadCodec struct{}

func (*typedNilPayloadCodec) Name() string { return "typed-nil-payload" }
func (*typedNilPayloadCodec) Marshal(any) (codec.Frame, error) {
	return nil, nil
}
func (*typedNilPayloadCodec) Unmarshal(codec.Frame) (any, error) {
	return nil, nil
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

func dialLoopbackEventually(t *testing.T, addr string) net.Conn {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		con, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			return con
		}
		lastErr = err
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client dial err: %v\n", lastErr)
	return nil
}

func mockClient(t *testing.T, addr string) {
	con := dialLoopbackEventually(t, addr)
	defer func() {
		_ = con.Close()
	}()

	msg := []byte("hello server!")
	header := make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(header, uint32(len(msg)))

	packet := append(header, msg...)

	_, err := con.Write(packet)
	if err != nil {
		t.Fatalf("client write msg err: %v\n", err)
	}

	header = make([]byte, binary.MaxVarintLen32)

	if _, err = stdio.ReadFull(con, header); err != nil {
		t.Fatalf("client read msg header err: %v\n", err)
	}

	length := binary.BigEndian.Uint32(header)
	body := make([]byte, length, length)

	if _, err = stdio.ReadFull(con, body); err != nil {
		t.Fatalf("client read msg body err: %v\n", err)
	}

	log.Infof("client read msg: %s", string(body))

	msg = []byte("i will close connection after 1 sec")
	header = make([]byte, binary.MaxVarintLen32)
	binary.BigEndian.PutUint32(header, uint32(len(msg)))
	_, _ = con.Write(append(header, msg...))

	time.Sleep(time.Second)
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

func newRouter() less.Router {
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

type shutdownAddr struct{}

func (shutdownAddr) Network() string { return "tcp" }
func (shutdownAddr) String() string  { return "127.0.0.1:18888" }

type shutdownConn struct {
	closed int32
}

func (c *shutdownConn) Read(buf []byte) (int, error)  { return 0, nil }
func (c *shutdownConn) Write(buf []byte) (int, error) { return len(buf), nil }
func (c *shutdownConn) IsActive() bool                { return atomic.LoadInt32(&c.closed) == 0 }
func (c *shutdownConn) Close() error {
	atomic.StoreInt32(&c.closed, 1)
	return nil
}
func (c *shutdownConn) LocalAddr() net.Addr  { return shutdownAddr{} }
func (c *shutdownConn) RemoteAddr() net.Addr { return shutdownAddr{} }

type blockingTransport struct {
	driver  transport.EventDriver
	started chan struct{}
	done    chan struct{}
	once    sync.Once
}

var _ transport.Transport = (*blockingTransport)(nil)

func newBlockingTransport() *blockingTransport {
	return &blockingTransport{
		started: make(chan struct{}),
		done:    make(chan struct{}),
	}
}

func (t *blockingTransport) Listen(addr string, driver transport.EventDriver) error {
	t.driver = driver
	t.once.Do(func() { close(t.started) })
	<-t.done
	return nil
}

func (t *blockingTransport) Dial(network, addr string, driver transport.EventDriver) error {
	t.driver = driver
	t.once.Do(func() { close(t.started) })
	<-t.done
	return nil
}

func (t *blockingTransport) Close() {
	select {
	case <-t.done:
	default:
		close(t.done)
	}
}

func TestServer_Shutdown_ClosesActiveConnections(t *testing.T) {
	blocking := newBlockingTransport()
	var trans transport.Transport = blocking
	srv := NewServer("127.0.0.1:18888", WithTransport(trans), WithRouter(newRouter()))

	srv.Run()

	select {
	case <-blocking.started:
	case <-time.After(time.Second):
		t.Fatal("server did not start transport listener")
	}

	conn := &shutdownConn{}
	if _, err := blocking.driver.OnConnect(context.Background(), conn); err != nil {
		t.Fatalf("OnConnect failed: %v", err)
	}

	srv.Shutdown(context.Background(), nil)

	deadline := time.After(200 * time.Millisecond)
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()

	for conn.IsActive() {
		select {
		case <-deadline:
			t.Fatal("expected Server.Shutdown to close active connections")
		case <-tick.C:
		}
	}
}

func TestNewServer_ClonesDefaultOptions(t *testing.T) {
	srvWithHooks := NewServer("127.0.0.1:18888",
		WithOnChannelClosed(deleteOnChannelClosed()),
		WithRouter(newRouter()),
	)
	srvWithoutHooks := NewServer("127.0.0.1:18889", WithRouter(newRouter()))

	if srvWithHooks.ops == srvWithoutHooks.ops {
		t.Fatal("expected NewServer to clone default options per instance")
	}

	if got := len(srvWithoutHooks.ops.transOptions); got != 1 {
		t.Fatalf("expected isolated transOptions for second server, got %d", got)
	}
}
