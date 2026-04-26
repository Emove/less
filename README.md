# Less

[![GoDoc][1]][2] [![Go Report Card][3]][4]

[1]: https://godoc.org/github.com/emove/less?status.svg
[2]: https://godoc.org/github.com/emove/less
[3]: https://goreportcard.com/badge/github.com/emove/less
[4]: https://goreportcard.com/report/github.com/emove/less

English | [中文](README_zh.md)

Less is a lightweight, composable network framework for Go. It focuses on a small public surface:

- `less`: core contracts such as `Channel`, `Router`, `Handler`, `Middleware`, and lifecycle hooks
- `server`: server endpoint assembly
- `client`: dial-side endpoint assembly
- `codec`: packet/payload codec abstractions plus built-in implementations under `codec/packet` and `codec/payload`
- `transport`: transport abstraction with the default TCP implementation under `transport/tcp`
- `log`: pluggable logging facade

The repository currently has a working server/client path, built-in codecs, two runnable examples, and end-to-end tests for the core lifecycle.

## Current Status

- Default transport is TCP via `transport/tcp`
- Default codec stack is variable-length packet codec + text payload codec
- Server and client both support `OnChannel`, `OnChannelClosed`, middleware, router, packet codec, and payload codec options
- `WithRouter(...)` is required on both server and client endpoints
- Client redial while a session is active is rejected by design

## When To Use Which Example

- `examples/chatroom`: start here if you want the smallest full request/response example with JSON messages and broadcast behavior
- `examples/device-gateway`: use this when you want to study a custom domain payload codec, authentication flow, heartbeats, telemetry, and command/ack interactions

## Package Overview

| Package | Purpose |
| --- | --- |
| `less` | Public interfaces and middleware/router contracts |
| `server` | Build and run a Less server |
| `client` | Dial a Less server and manage the active channel |
| `codec/packet` | Framing strategies such as variable-length, fixed-length, and delimiter-based packets |
| `codec/payload` | Payload serialization such as text and JSON |
| `transport` | Transport abstraction used by the framework |
| `transport/tcp` | Built-in Go `net` based TCP transport |
| `log` | Logging interface and default logger |

## Built-in Codecs

### Packet codecs

- `packet.NewVariableLengthCodec()`
- `packet.NewFixedLengthCodec(length)`
- `packet.NewDelimiterCodec(delimiter, maxLength, ...options)`

### Payload codecs

- `payload.NewTextCodec()`
- `payload.NewJSONCodec()`
- `payload.NewJSONCodecWithType(sample)`

## Install

```shell
go get github.com/emove/less
```

## Quick Start

The minimal server setup is: choose a transport, choose codecs, provide a router, then run the server.

```go
package main

import (
	"context"
	"fmt"

	"github.com/emove/less"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
	"github.com/emove/less/server"
)

func main() {
	srv := server.NewServer(
		":8080",
		server.WithPacketCodec(packet.NewVariableLengthCodec()),
		server.WithPayloadCodec(payload.NewTextCodec()),
		server.WithRouter(func(ctx context.Context, ch less.Channel, msg interface{}) (less.Handler, error) {
			return func(_ context.Context, ch less.Channel, msg interface{}) error {
				text := msg.(string)
				fmt.Println("recv:", text)
				return ch.Write("echo: " + text)
			}, nil
		}),
	)

	srv.Run()
	defer srv.Shutdown(context.Background(), nil)

	select {}
}
```

Notes:

- `server.Run()` starts the transport asynchronously
- `WithRouter(...)` is required
- If you do not override codecs, the server defaults to variable-length packets and text payloads

## Client Quick Start

The dial-side setup mirrors the server: choose codecs, provide a router for inbound messages, dial, then use the active channel.

```go
package main

import (
	"context"
	"fmt"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
)

func main() {
	cli := client.NewClient(
		"tcp",
		"127.0.0.1:8080",
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(payload.NewTextCodec()),
		client.WithRouter(func(ctx context.Context, ch less.Channel, msg interface{}) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, msg interface{}) error {
				fmt.Println("recv:", msg.(string))
				return nil
			}, nil
		}),
	)
	defer cli.Close(nil)

	if err := cli.Dial(context.Background()); err != nil {
		panic(err)
	}

	ch := cli.Channel()
	if ch == nil {
		panic("dial succeeded without an active channel")
	}

	if err := ch.Write("hello"); err != nil {
		panic(err)
	}
}
```

Notes:

- `client.Channel()` returns the current active channel after a successful `Dial(...)`
- `client.WithRouter(...)` is required because inbound messages still go through the same decode -> route -> handler path
- Re-dialing while the current session is still active returns an error

## Endpoint Options

### Server options

- `server.WithTransport(...)`
- `server.WithPacketCodec(...)`
- `server.WithPayloadCodec(...)`
- `server.WithRouter(...)`
- `server.WithOnChannel(...)`
- `server.WithOnChannelClosed(...)`
- `server.WithInboundMiddleware(...)`
- `server.WithOutboundMiddleware(...)`
- `server.WithShutdownHooks(...)`
- `server.MaxChannelSize(...)`
- `server.MaxSendMessageSize(...)`
- `server.MaxReceiveMessageSize(...)`

### Client options

- `client.WithTransport(...)`
- `client.WithPacketCodec(...)`
- `client.WithPayloadCodec(...)`
- `client.WithRouter(...)`
- `client.WithOnChannel(...)`
- `client.WithOnChannelClosed(...)`
- `client.WithInboundMiddleware(...)`
- `client.WithOutboundMiddleware(...)`
- `client.MaxSendMessageSize(...)`
- `client.MaxReceiveMessageSize(...)`

## Run The Example

The repository includes two runnable examples.

### Chatroom

Start the server:

```shell
go run ./examples/chatroom/server -addr 127.0.0.1:8888
```

Start one or more clients in separate terminals:

```shell
go run ./examples/chatroom/client -addr 127.0.0.1:8888
```

The chatroom example uses:

- `packet.NewVariableLengthCodec()` for framing
- `chat.NewJSONCodec()` for payload serialization
- `OnChannel` / `OnChannelClosed` hooks to manage join and leave events
- a router to dispatch `set_name`, `chat`, and system messages

Relevant source files:

- `examples/chatroom/server`
- `examples/chatroom/client`
- `examples/chatroom/chat`

### Device gateway

Start the gateway server:

```shell
go run ./examples/device-gateway/server -addr 127.0.0.1:9000
```

Start a simulated device:

```shell
go run ./examples/device-gateway/device -addr 127.0.0.1:9000 -device-id dev-001 -secret demo-secret
```

The device-gateway example uses:

- the standard variable-length packet codec for outer framing
- a custom payload codec under `examples/device-gateway/protocol`
- `OnChannel` / `OnChannelClosed` to register and retire device sessions
- a typed protocol with auth, heartbeat, telemetry, command, and command ack messages

Relevant source files:

- `examples/device-gateway/server`
- `examples/device-gateway/device`
- `examples/device-gateway/protocol`
- `examples/device-gateway/README.md` for a protocol-oriented walkthrough

## Request Lifecycle

The current request path is:

1. transport accepts or dials a connection
2. `OnChannel` hooks run when the channel is activated
3. packet codec decodes bytes into a frame
4. payload codec unmarshals the frame into a message
5. inbound middleware runs
6. router selects a handler
7. handler executes business logic
8. outbound writes go through outbound middleware, payload marshal, packet encode, and connection flush
9. `OnChannelClosed` hooks run when the channel closes

Middleware execution order is:

- inbound: global inbound -> channel inbound -> router
- outbound: channel outbound -> global outbound -> write handler

## Transport Model

Less uses a single `transport.Transport` abstraction:

- `Listen(addr, driver)`
- `Dial(network, addr, driver)`
- `Close()`

The repository currently ships with `transport/tcp` only. If you want another backend, implement the `transport.Transport` interface and pass it with `WithTransport(...)`.

### Built-in TCP transport options

The default transport is created with `tcp.New()`. To customize it, pass `server.WithTransport(tcp.New(...))` or `client.WithTransport(tcp.New(...))`.

Supported TCP options include:

- `tcp.WithNetwork(tcp.TCP | tcp.TCP4 | tcp.TCP6)`
- `tcp.WithTimeout(duration)` for dial timeout
- `tcp.WithKeepalive(bool)`
- `tcp.WithKeepalivePeriod(duration)`
- `tcp.WithLinger(int)`
- `tcp.WithNoDelay(bool)`
