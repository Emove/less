# less Client Peer Design

## Context

The current public framework shape is server-centric. `server.Run()` assembles options, creates an engine transport handler, and hands it to the transport layer. After a connection is established, however, the runtime behavior is already close to a peer model:

- a `Channel` is created per connection
- inbound messages flow through packet decode, payload unmarshal, inbound middleware, and router
- outbound messages flow through channel write, outbound middleware, payload marshal, packet encode, and transport write
- lifecycle hooks (`OnChannel`, `OnChannelClosed`) already hang off the channel rather than the server itself

The missing piece is not a lightweight sender-style client. The missing piece is a first-class peer endpoint for the active dial side of the same framework model.

## Goal

Define a `client` package that is behaviorally symmetric with `server` after connection establishment:

- both server and client support `middleware`
- both server and client support `router`
- both server and client support `OnChannel` and `OnChannelClosed`
- both server and client support packet and payload codec customization
- the only intentional semantic difference is connection establishment:
  - server listens for inbound connections
  - client dials outbound connections

## Non-Goals

- automatic reconnect in the first version
- connection pooling or multiplexing in the first version
- introducing a new public session abstraction other than `less.Channel`
- splitting client into a simplified request-response API path that bypasses router or middleware

## Design Principles

1. Server and client are peers, not separate message models.
2. Public entrypoints may differ, but the connected channel behavior must be identical.
3. `less.Channel` remains the core public connection contract.
4. Transport and engine boundaries must stay explicit and not depend on hidden concrete downcasts.
5. Client must report dial failure synchronously.

## Proposed Architecture

### 1. Public API remains split into `server` and `client`

Keep separate public packages:

- `server`
- `client`

This keeps user intent obvious at the entrypoint level. A caller still chooses whether it is the passive listener side or the active dial side. The runtime contract after connection establishment is shared.

### 2. Internal runtime becomes endpoint-oriented

`internal/engine` should expose a transport event driver that represents a generic connected endpoint rather than a server-only transport handler.

This endpoint handler owns:

- channel creation
- outbound write path setup
- `OnChannel` execution
- inbound decode and dispatch
- connection close handling
- channel tracking for lifecycle and cleanup

The existing server-specific naming should be removed from this runtime component because the behavior is not inherently server-specific.

### 3. Transport becomes a unified public abstraction

The public transport abstraction should support both listen and dial flows. The same transport instance type should be usable by both `server` and `client`.

Required shape:

```go
type Transport interface {
    Listen(addr string, driver EventDriver) error
    Dial(network, addr string, driver EventDriver) error
    Close()
}
```

This removes the current asymmetry where the TCP implementation already has dial capability but the public transport contract does not.

### 4. Channel remains the shared session contract

The client package should not introduce a separate connection/session interface. Once connected, business code should interact with the same `less.Channel` that server-side code uses.

This preserves:

- consistent middleware behavior
- consistent router invocation semantics
- consistent channel lifecycle hooks
- one stable public message path across both sides

## Public API Proposal

### Server

```go
type Server struct { ... }

func NewServer(addr string, ops ...Option) *Server
func (srv *Server) Run()
func (srv *Server) Shutdown(ctx context.Context, err error)
```

### Client

```go
type Client struct { ... }

func NewClient(network, addr string, ops ...Option) *Client
func (cli *Client) Dial(ctx context.Context) error
func (cli *Client) Channel() less.Channel
func (cli *Client) Close(err error)
```

Optional convenience method:

```go
func (cli *Client) Write(msg any) error
```

If added, `Write` is only sugar over the currently active channel. It must not become a parallel message path with its own behavior.

## Shared Options

Server and client should expose the same functional options where meaningful:

- `WithTransport(...)`
- `WithOnChannel(...)`
- `WithOnChannelClosed(...)`
- `WithInboundMiddleware(...)`
- `WithOutboundMiddleware(...)`
- `WithRouter(...)`
- `WithPacketCodec(...)`
- `WithPayloadCodec(...)`
- `MaxSendMessageSize(...)`
- `MaxReceiveMessageSize(...)`

`MaxChannelSize(...)` remains server-only unless the framework later adds multi-connection client behavior. For a single-connection manual-dial client, it has no clear meaning.

## Runtime Flow

### Server flow

1. `server.Run()` builds a generic endpoint handler from options.
2. `transport.Listen(addr, handler)` accepts inbound connections.
3. On each accepted connection, the endpoint handler creates a channel and activates it.
4. Message processing then follows the shared runtime path.

### Client flow

1. `client.Dial(ctx)` builds the same generic endpoint handler from options.
2. `transport.Dial(network, addr, handler)` establishes an outbound connection.
3. On successful connection, the endpoint handler creates a channel and activates it.
4. The client stores the active channel.
5. Message processing then follows the same shared runtime path as server.

### Shared inbound path

1. `PacketCodec.Decode`
2. `PayloadCodec.Unmarshal`
3. inbound middleware chain
4. router-selected handler

### Shared outbound path

1. `Channel.Write`
2. outbound middleware chain
3. `PayloadCodec.Marshal`
4. `PacketCodec.Encode`
5. transport write

### Shared close path

1. transport reports connection closed
2. endpoint handler closes the channel
3. `OnChannelClosed` fires exactly once

## Router Semantics

Client and server should share router semantics. The client must not gain a special message callback path that bypasses router by default.

Reasoning:

- the user requirement is explicit symmetry
- a special client-only path would create two mental models
- middleware and router ordering would become harder to reason about and test

If a future simplification is desired, it should be expressed as a helper that is implemented on top of the router contract, not as a second runtime path.

## Error Handling

### Dial

`Client.Dial(ctx)` must return errors synchronously for:

- transport dial failure
- `OnChannel` rejection
- codec or handler setup failure before the channel becomes active

This is intentionally different from `Server.Run()`, which can tolerate asynchronous listener lifetime.

### Runtime errors

Once connected:

- decode/unmarshal errors close the channel
- outbound encode/write errors close the channel
- transport close propagates to channel close
- `OnChannelClosed` is the single close notification point

### Close idempotence

Both client-owned shutdown and transport-owned disconnect must converge on the same channel close path so callbacks remain single-fire.

## Testing Requirements

The client design is only acceptable if behavior is testable in the same way as server behavior.

Minimum integration coverage:

1. client dial succeeds and triggers `OnChannel`
2. client sends a message through outbound middleware and codec chain
3. client receives a message through decode, inbound middleware, and router chain
4. client close triggers `OnChannelClosed` exactly once
5. remote disconnect triggers `OnChannelClosed` exactly once
6. shared ordering is asserted:
   - inbound: `globalInbound -> chInbound -> router`
   - outbound: `chOutbound -> globalOutbound -> writeHandler`

## Trade-offs

### Chosen approach

Use a shared endpoint runtime with separate thin public `server` and `client` packages.

### Why not a client-specific lightweight callback model

It would make the client easier to sketch quickly, but it would violate the stated requirement that server and client are peers with the same framework features.

### Why not a single public `Peer` type

It would be conceptually pure, but it would make the user-facing API less obvious. `server` and `client` remain clearer public entrypoints even when they share the same internal runtime model.

## Migration Notes

Expected internal changes:

- rename server-specific engine handler naming to endpoint-oriented naming
- unify transport interface to include dial capability
- add codec options to the public server package
- create a new public client package mirroring server option surface
- ensure client stores and exposes the active `less.Channel`

These are targeted structural changes in support of a single model, not a general refactor.

## Success Criteria

This design is successful when:

- users can configure client and server with the same channel-processing features
- client and server share one runtime message model
- transport boundaries are explicit and public abstractions are sufficient for both sides
- connected business logic can treat server-side and client-side channels the same way
- the only meaningful behavioral difference between server and client is `Listen` versus `Dial`
