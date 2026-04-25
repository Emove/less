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

The repository currently has a working server/client path, built-in codecs, a runnable chatroom example, and end-to-end tests for the core lifecycle.

## Current Status

- Default transport is TCP via `transport/tcp`
- Default codec stack is variable-length packet codec + text payload codec
- Server and client both support `OnChannel`, `OnChannelClosed`, middleware, router, packet codec, and payload codec options
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

## Run The Example

The repository includes a runnable chatroom example under `examples/chatroom`.

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

## Testing

Run the full test suite:

```shell
go test ./...
```

Useful focused suites:

```shell
go test ./server ./client ./test/e2e
```

The current tests cover:

- server and client lifecycle
- codec wiring on read/write paths
- channel hooks
- runnable chatroom behavior
- end-to-end connection/message/close flows

## Notes For Contributors

- Treat `internal/...` as unstable implementation detail
- Public examples should use packages under `less`, `server`, `client`, `codec`, `transport`, and `log`
- If you change public behavior, update both `README.md` and `README_zh.md`
