package client

import (
	"context"
	"errors"
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
	network         string
	addr            string
	ctx             context.Context
	cancelFunc      context.CancelFunc
	ops             *clientOptions
	handler         engine.TransHandler
	channel         less.Channel
	activeSessionID uint64
	nextSessionID   uint64

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

var (
	errClientSessionActive  = errors.New("client already has an active session")
	errClientSessionRetired = errors.New("client session retired during dial")
)

var newEndpointHandler = engine.NewEndpointHandler

// Dial creates the endpoint handler and dials the remote transport synchronously.
func (cli *Client) Dial(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	cli.mu.Lock()
	if cli.activeSessionID != 0 || cli.handler != nil || channelIsActive(cli.channel) {
		cli.mu.Unlock()
		return errClientSessionActive
	}
	sessionID := cli.nextSessionID + 1
	cli.nextSessionID = sessionID
	cli.activeSessionID = sessionID
	dialCtx, cancelFunc := context.WithCancel(ctx)
	cli.ctx = dialCtx
	cli.cancelFunc = cancelFunc
	cli.channel = nil
	cli.handler = nil
	cli.mu.Unlock()

	options := make([]engine.Option, 0, len(cli.ops.transOptions)+2)
	options = append(options,
		engine.AddOnChannel(cli.captureChannel(sessionID)),
		engine.AddOnChannelClosed(cli.clearClosedChannel(sessionID)),
	)
	options = append(options, cli.ops.transOptions...)

	handler := newEndpointHandler(dialCtx, options...)

	if !cli.publishHandlerIfCurrent(sessionID, handler) {
		_ = handler.Close()
		cancelFunc()
		return sessionRetiredErr(dialCtx)
	}

	if err := cli.ops.transport.Dial(cli.network, cli.addr, handler); err != nil {
		cli.closeSessionIfCurrent(sessionID, err, false)
		return err
	}
	if !cli.sessionOwnsHandler(sessionID, handler) {
		_ = handler.Close()
		cancelFunc()
		return sessionRetiredErr(dialCtx)
	}

	go cli.closeWhenDialContextDone(sessionID, ctx, dialCtx)

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
	cli.closeSession(err, true)
}

func (cli *Client) closeWhenDialContextDone(sessionID uint64, parentCtx, dialCtx context.Context) {
	if dialCtx == nil || dialCtx.Done() == nil {
		return
	}
	<-dialCtx.Done()
	if err := parentCtx.Err(); err != nil {
		cli.closeSessionIfCurrent(sessionID, err, false)
	}
}

func (cli *Client) closeSessionIfCurrent(sessionID uint64, err error, closeTransport bool) bool {
	cli.mu.Lock()
	if cli.activeSessionID != sessionID {
		cli.mu.Unlock()
		return false
	}
	handler := cli.handler
	cli.handler = nil
	ch := cli.channel
	cli.channel = nil
	cancelFunc := cli.cancelFunc
	cli.cancelFunc = nil
	cli.ctx = nil
	cli.activeSessionID = 0
	cli.mu.Unlock()

	if ch != nil {
		ch.Close(err)
	}
	if handler != nil {
		_ = handler.Close()
	}
	if closeTransport {
		cli.ops.transport.Close()
	}
	if cancelFunc != nil {
		cancelFunc()
	}
	return true
}

func (cli *Client) closeSession(err error, closeTransport bool) {
	cli.mu.Lock()
	handler := cli.handler
	cli.handler = nil
	ch := cli.channel
	cli.channel = nil
	cancelFunc := cli.cancelFunc
	cli.cancelFunc = nil
	cli.ctx = nil
	cli.activeSessionID = 0
	cli.mu.Unlock()

	if ch != nil {
		ch.Close(err)
	}
	if handler != nil {
		_ = handler.Close()
	}
	if closeTransport {
		cli.ops.transport.Close()
	}
	if cancelFunc != nil {
		cancelFunc()
	}
}

func (cli *Client) publishHandlerIfCurrent(sessionID uint64, handler engine.TransHandler) bool {
	cli.mu.Lock()
	defer cli.mu.Unlock()
	if cli.activeSessionID != sessionID {
		return false
	}
	cli.handler = handler
	return true
}

func (cli *Client) sessionOwnsHandler(sessionID uint64, handler engine.TransHandler) bool {
	cli.mu.RLock()
	defer cli.mu.RUnlock()
	return cli.activeSessionID == sessionID && cli.handler == handler
}

func (cli *Client) captureChannel(sessionID uint64) less.OnChannel {
	return func(ctx context.Context, ch less.Channel) (context.Context, error) {
		cli.mu.Lock()
		if cli.activeSessionID == sessionID {
			cli.channel = ch
		}
		cli.mu.Unlock()
		return ctx, nil
	}
}

func (cli *Client) clearClosedChannel(sessionID uint64) less.OnChannelClosed {
	return func(_ context.Context, _ less.Channel, _ error) {
		cli.retireSessionIfCurrent(sessionID)
	}
}

func (cli *Client) retireSessionIfCurrent(sessionID uint64) bool {
	cli.mu.Lock()
	if cli.activeSessionID != sessionID {
		cli.mu.Unlock()
		return false
	}
	handler := cli.handler
	cli.handler = nil
	cli.channel = nil
	cancelFunc := cli.cancelFunc
	cli.cancelFunc = nil
	cli.ctx = nil
	cli.activeSessionID = 0
	cli.mu.Unlock()

	if handler != nil {
		_ = handler.Close()
	}
	if cancelFunc != nil {
		cancelFunc()
	}
	return true
}

func sessionRetiredErr(ctx context.Context) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return errClientSessionRetired
}

// WithTransport sets the transport used by the client.
func WithTransport(transport transport.Transport) CliOption {
	if valueIsNil(transport) {
		panic("transport can not be nil")
	}
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
	if valueIsNil(c) {
		panic("packet codec can not be nil")
	}
	return func(ops *clientOptions) {
		ops.transOptions = append(ops.transOptions, engine.WithPacketCodec(c))
	}
}

// WithPayloadCodec sets the payload codec used by the client endpoint.
func WithPayloadCodec(c codec.PayloadCodec) CliOption {
	if valueIsNil(c) {
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

func valueIsNil(v any) bool {
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

func channelIsActive(ch less.Channel) bool {
	return ch != nil && ch.IsActive()
}
