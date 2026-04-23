package client

import (
	"context"
	"reflect"
	"sync"

	"github.com/emove/less"
	"github.com/emove/less/codec"
	engine "github.com/emove/less/internal/engine"
	"github.com/emove/less/transport"
	"github.com/emove/less/transport/tcp"
)

// Client is the dial-side peer for a less endpoint.
type Client struct {
	network    string
	addr       string
	ctx        context.Context
	cancelFunc context.CancelFunc
	ops        *clientOptions
	handler    engine.TransHandler
	channel    less.Channel

	mu sync.RWMutex
}

type clientOptions struct {
	transport    transport.Transport
	transOptions []engine.Option
}

// CliOption configures a Client.
type CliOption func(*clientOptions)

// NewClient creates a dial-side client with the provided network and address.
func NewClient(network, addr string, op ...CliOption) *Client {
	ops := defaultClientOptions()
	for _, o := range op {
		o(ops)
	}

	ctx, cancelFunc := context.WithCancel(context.Background())

	return &Client{
		network:    network,
		addr:       addr,
		ctx:        ctx,
		cancelFunc: cancelFunc,
		ops:        ops,
	}
}

func defaultClientOptions() *clientOptions {
	return &clientOptions{
		transport: tcp.New(),
	}
}

// Dial creates the endpoint handler and dials the remote transport synchronously.
func (cli *Client) Dial(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	cli.mu.Lock()
	if cli.cancelFunc != nil {
		cli.cancelFunc()
	}
	cli.ctx, cli.cancelFunc = context.WithCancel(ctx)
	cli.channel = nil
	cli.mu.Unlock()

	options := make([]engine.Option, 0, len(cli.ops.transOptions)+2)
	options = append(options,
		engine.AddOnChannel(cli.captureChannel),
		engine.AddOnChannelClosed(cli.clearClosedChannel),
	)
	options = append(options, cli.ops.transOptions...)

	handler := engine.NewEndpointHandler(cli.ctx, options...)

	cli.mu.Lock()
	cli.handler = handler
	cli.mu.Unlock()

	if err := cli.ops.transport.Dial(cli.network, cli.addr, handler); err != nil {
		_ = handler.Close()
		cli.mu.Lock()
		cli.channel = nil
		cli.handler = nil
		cancelFunc := cli.cancelFunc
		cli.cancelFunc = nil
		cli.ctx = nil
		cli.mu.Unlock()
		if cancelFunc != nil {
			cancelFunc()
		}
		return err
	}

	return nil
}

// Channel returns the currently active client channel, if any.
func (cli *Client) Channel() less.Channel {
	cli.mu.RLock()
	defer cli.mu.RUnlock()
	return cli.channel
}

// Close closes the handler, transport, context, and active channel if present.
func (cli *Client) Close(err error) {
	cli.mu.Lock()
	handler := cli.handler
	cli.handler = nil
	ch := cli.channel
	cli.channel = nil
	cancelFunc := cli.cancelFunc
	cli.cancelFunc = nil
	cli.ctx = nil
	cli.mu.Unlock()

	if handler != nil {
		_ = handler.Close()
	}
	cli.ops.transport.Close()
	if cancelFunc != nil {
		cancelFunc()
	}
	if ch != nil {
		ch.Close(err)
	}
}

func (cli *Client) captureChannel(ctx context.Context, ch less.Channel) (context.Context, error) {
	cli.mu.Lock()
	cli.channel = ch
	cli.mu.Unlock()
	return ctx, nil
}

func (cli *Client) clearClosedChannel(_ context.Context, ch less.Channel, _ error) {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if cli.channel == ch {
		cli.channel = nil
	}
}

// WithTransport sets the transport used by the client.
func WithTransport(transport transport.Transport) CliOption {
	return func(ops *clientOptions) {
		ops.transport = transport
	}
}

// WithOnChannel adds channel activation hooks for the client endpoint.
func WithOnChannel(onChannel ...less.OnChannel) CliOption {
	return func(ops *clientOptions) {
		if len(onChannel) > 0 {
			ops.transOptions = append(ops.transOptions, engine.AddOnChannel(onChannel...))
		}
	}
}

// WithOnChannelClosed adds channel closed hooks for the client endpoint.
func WithOnChannelClosed(onChannelClosed ...less.OnChannelClosed) CliOption {
	return func(ops *clientOptions) {
		if len(onChannelClosed) > 0 {
			ops.transOptions = append(ops.transOptions, engine.AddOnChannelClosed(onChannelClosed...))
		}
	}
}

// WithInboundMiddleware adds inbound middleware to the client endpoint.
func WithInboundMiddleware(mws ...less.Middleware) CliOption {
	return func(ops *clientOptions) {
		if len(mws) > 0 {
			ops.transOptions = append(ops.transOptions, engine.AddInboundMiddleware(mws...))
		}
	}
}

// WithOutboundMiddleware adds outbound middleware to the client endpoint.
func WithOutboundMiddleware(mws ...less.Middleware) CliOption {
	return func(ops *clientOptions) {
		if len(mws) > 0 {
			ops.transOptions = append(ops.transOptions, engine.AddOutboundMiddleware(mws...))
		}
	}
}

// WithRouter sets the router used by the client endpoint.
func WithRouter(router less.Router) CliOption {
	return func(ops *clientOptions) {
		ops.transOptions = append(ops.transOptions, engine.WithRouter(router))
	}
}

// WithPacketCodec sets the packet codec used by the client endpoint.
func WithPacketCodec(c codec.PacketCodec) CliOption {
	if codecIsNil(c) {
		panic("packet codec can not be nil")
	}
	return func(ops *clientOptions) {
		ops.transOptions = append(ops.transOptions, engine.WithPacketCodec(c))
	}
}

// WithPayloadCodec sets the payload codec used by the client endpoint.
func WithPayloadCodec(c codec.PayloadCodec) CliOption {
	if codecIsNil(c) {
		panic("payload codec can not be nil")
	}
	return func(ops *clientOptions) {
		ops.transOptions = append(ops.transOptions, engine.WithPayloadCodec(c))
	}
}

// MaxSendMessageSize sets the max outbound message size for the client endpoint.
func MaxSendMessageSize(size uint32) CliOption {
	return func(ops *clientOptions) {
		ops.transOptions = append(ops.transOptions, engine.MaxSendMessageSize(size))
	}
}

// MaxReceiveMessageSize sets the max inbound message size for the client endpoint.
func MaxReceiveMessageSize(size uint32) CliOption {
	return func(ops *clientOptions) {
		ops.transOptions = append(ops.transOptions, engine.MaxReceiveMessageSize(size))
	}
}

func codecIsNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
