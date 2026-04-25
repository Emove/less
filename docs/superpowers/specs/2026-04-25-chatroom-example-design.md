# less Chatroom Example Design

## Context

`less` already has a working TCP server-client path:

- `server.NewServer(...).Run()` listens through the public `transport.Transport` interface.
- `client.NewClient(...).Dial(ctx)` creates an active client-side endpoint.
- Both sides use `less.Channel` for lifecycle and writes.
- The engine composes packet codec, payload codec, middleware, router, and handler.
- `test/e2e` already verifies real connection establishment, bidirectional message exchange, and close callbacks.

This makes a small chatroom example a good fit for the current framework. It can demonstrate the framework's intended usage without changing the core runtime.

## Goal

Build a minimal, runnable chatroom usage example based on the current `less` framework.

The example should show:

- how to start a `less/server` endpoint;
- how to connect with `less/client`;
- how `OnChannel` and `OnChannelClosed` manage connection lifecycle;
- how JSON payloads become typed application messages;
- how `Router` selects handlers by message type;
- how handlers broadcast by calling `Channel.Write`.

The primary audience is a developer reading the repository and trying to understand how to build a small application on top of `less`.

## Non-Goals

- Do not implement WebSocket support.
- Do not change framework core code.
- Do not introduce third-party production dependencies.
- Do not implement multiple rooms, private messages, history, authentication, reconnect, persistence, or moderation.
- Do not turn the chatroom into a reusable framework package.
- Do not rely on fixed ports in tests.

## User-Facing Shape

The example lives under `examples/chatroom/` and contains two runnable programs:

- `examples/chatroom/server`
- `examples/chatroom/client`

Expected manual usage:

```shell
go run ./examples/chatroom/server -addr 127.0.0.1:8888
go run ./examples/chatroom/client -addr 127.0.0.1:8888
```

The client asks for a nickname first. After that, each input line is sent as a chat message.

Terminal output format:

```text
[system] alice joined
[alice] hello
```

## Message Protocol

The example uses JSON payload messages over the existing length-prefixed packet codec.

Client-to-server messages:

```json
{"type":"set_name","name":"alice"}
{"type":"chat","text":"hello"}
```

Server-to-client messages:

```json
{"type":"system","text":"alice joined"}
{"type":"chat","name":"alice","text":"hello"}
```

The protocol has exactly three message types:

- `set_name`: client sets or changes its display name.
- `chat`: client sends a chat line, or server broadcasts a chat line.
- `system`: server sends lifecycle or validation messages.

Unknown message types are handled as protocol errors for that message and should receive a `system` response when possible.

## Architecture

Use a lightweight layered example instead of putting all behavior in one handler.

### Shared Chat Package

Path: `examples/chatroom/chat`

Responsibilities:

- define the message model;
- provide a JSON payload codec compatible with `codec.PayloadCodec`;
- expose helpers for constructing system and chat messages if that keeps the example readable.

This package exists only for the example. It should not become part of the public framework API.

### Server Program

Path: `examples/chatroom/server`

Responsibilities:

- parse `-addr`;
- configure `server.NewServer`;
- use `packet.NewVariableLengthCodec()`;
- use the example JSON payload codec;
- create and pass a `hub`;
- wire `OnChannel`, `OnChannelClosed`, and `Router`;
- block until interrupted or process termination.

The server should keep the framework setup easy to read in `main.go`. Chatroom state and handlers should be split out when that improves clarity.

### Client Program

Path: `examples/chatroom/client`

Responsibilities:

- parse `-addr`;
- ask for a nickname;
- configure `client.NewClient`;
- connect to the server;
- send `set_name`;
- read stdin lines and send `chat`;
- print inbound `system` and `chat` messages.

The client should remain a terminal client. There is no browser UI.

### Hub

The hub is server-local state. It owns:

- a `sync.RWMutex`;
- a map from `less.Channel` to `session`;
- a monotonic connection id generated with `sync/atomic`;
- register, unregister, set name, and broadcast operations.

The hub should not know about packet codecs or payload codecs. It only handles sessions and channels.

### Session

Each session stores:

- `id`;
- `name`;
- `channel`.

A new session starts unnamed. The server accepts the connection in `OnChannel`, then waits for `set_name` before allowing normal chat messages.

## Data Flow

Server-side flow:

1. A client connects.
2. `OnChannel` creates an unnamed session in the hub.
3. The client sends `set_name`.
4. The JSON payload codec unmarshals bytes into a message object.
5. `Router` selects `setNameHandler`.
6. `setNameHandler` validates and stores the nickname.
7. The hub broadcasts a `system` join message.
8. Later `chat` messages route to `chatHandler`.
9. `chatHandler` validates that the session has a name and broadcasts the message.
10. `OnChannelClosed` removes the session and broadcasts a leave message if the user had joined.

Client-side flow:

1. The client dials the server.
2. The user enters a nickname.
3. The client writes a `set_name` message.
4. Each subsequent stdin line is written as a `chat` message.
5. Inbound messages route to one print handler and are written to stdout.

## Error Handling

Keep error behavior small and explicit:

- malformed JSON closes the connection, because the peer is not following the protocol;
- empty nickname returns a `system` error message and keeps the connection open;
- `chat` before `set_name` returns a `system` error message and keeps the connection open;
- unknown message type returns a `system` error message when possible;
- failed broadcast write closes that target channel and lets `OnChannelClosed` clean up state;
- server shutdown uses the existing `Shutdown` path.

The example should avoid logging-based correctness. Tests should observe messages and lifecycle events directly.

## Testing

Add example-level integration coverage. The test should use public APIs and dynamic ports.

Minimum scenario:

1. Reserve a dynamic local TCP address, then start the chat server on that address.
2. Connect two `less/client` clients using the same JSON payload codec.
3. Client A sends `set_name`.
4. Client B sends `set_name`.
5. Client A sends `chat`.
6. Assert Client B receives the expected broadcast.
7. Close Client A.
8. Assert Client B receives the expected leave `system` message.

The test should use deadlines for every wait and should not depend on fixed sleeps or log output.

## Implementation Boundaries

This feature should not modify:

- `less.go`;
- `server/`;
- `client/`;
- `transport/`;
- `internal/engine`;
- `internal/channel`;
- existing codec packages.

If an implementation uncovers a framework bug, that bug should be handled as a separate fix or explicitly added to the implementation plan.

## Acceptance Criteria

- `go run ./examples/chatroom/server -addr 127.0.0.1:8888` starts a server.
- `go run ./examples/chatroom/client -addr 127.0.0.1:8888` connects and can send messages.
- Two terminal clients can exchange broadcast chat messages.
- Join and leave system messages are visible.
- The example has an integration test for the core two-client flow.
- `go test ./...` passes.
- No third-party production dependency is added.
