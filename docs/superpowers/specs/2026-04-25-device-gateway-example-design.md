# less Device Gateway Example Design

## Context

`less` is already strongest when used as a long-lived message endpoint framework rather than an HTTP-style server.
The current repository exposes a clear runtime path:

- `server.NewServer(...).Run()` for the passive endpoint;
- `client.NewClient(...).Dial(ctx)` for the active endpoint;
- `less.Channel` for lifecycle and bidirectional writes;
- `Router` for message-to-handler dispatch;
- `codec.PacketCodec` for framing and `codec.PayloadCodec` for message serialization.

The repository also now includes a full `examples/chatroom/` flow, but that example teaches a JSON happy path more than it teaches protocol authoring. The next example should stay close to a familiar real-world business scenario while making the protocol boundary explicit.

## Goal

Add a runnable device gateway example that demonstrates both:

- a realistic long-connection business flow;
- how to define and integrate a custom message protocol on top of `less`.

The example should teach:

- device authentication after connect;
- heartbeat-driven liveness management;
- telemetry uplink from device to server;
- command downlink from server to device;
- command acknowledgment from device back to server;
- lifecycle cleanup through `OnChannelClosed`;
- how a custom payload codec turns protocol bytes into typed business messages.

The primary audience is a developer evaluating whether `less` can host a custom TCP-style device protocol without hiding the connection lifecycle behind a large framework.

## Non-Goals

- Do not change the core `less` runtime for this example.
- Do not introduce third-party production dependencies.
- Do not add reconnect, offline delivery, retry queues, persistence, or a web console.
- Do not turn the example into a general IoT platform.
- Do not introduce a second control-plane application unless required by a later follow-up.
- Do not make packet framing itself the teaching focus of this example.

## Recommended Shape

The example lives under `examples/device-gateway/` and contains three focused parts:

- `examples/device-gateway/protocol`
- `examples/device-gateway/server`
- `examples/device-gateway/device`

Expected manual usage:

```shell
go run ./examples/device-gateway/server -addr 127.0.0.1:9000
go run ./examples/device-gateway/device -addr 127.0.0.1:9000 -device-id dev-001 -secret demo-secret
```

The device program acts as a simulator, not a generic SDK. It should authenticate, emit heartbeats, emit a sample telemetry message, receive one command, send one command acknowledgment, then exit or wait for shutdown depending on the implementation plan.

## Protocol Design

This example should use:

- the existing `packet.NewVariableLengthCodec()` for outer frame boundaries;
- a custom payload codec for the device protocol itself.

This keeps the teaching focus on message protocol authoring instead of forcing the example to explain framing and payload concerns at the same time.

### Wire Shape

Inside each variable-length packet, the payload bytes should follow this layout:

```text
| version(1) | type(1) | request_id(4) | body_length(4) | body(N) |
```

Field semantics:

- `version`: protocol version, initially fixed to `1`;
- `type`: message kind discriminator;
- `request_id`: correlates downlink commands with device acknowledgments; `0` is allowed for uncorrelated messages;
- `body_length`: byte length of the body section;
- `body`: ASCII key-value text formatted as `k=v;k=v`.

### Message Types

The protocol should start with exactly six message types:

- `auth`
- `auth_ack`
- `heartbeat`
- `telemetry`
- `command`
- `command_ack`

### Example Bodies

- `auth`: `device_id=dev-001;secret=demo-secret`
- `auth_ack`: `status=ok`
- `heartbeat`: `ts=1714032000`
- `telemetry`: `temp=23.4;humidity=48`
- `command`: `name=reboot`
- `command_ack`: `status=ok`

This protocol is intentionally mixed-format:

- the header is binary and explicit enough to feel like a real device protocol;
- the body is textual enough to remain readable in logs, tests, and docs.

## Architecture

### `protocol` Package

Path: `examples/device-gateway/protocol`

Responsibilities:

- define message type constants;
- define typed message structs or a typed message interface;
- encode and decode the binary header;
- encode and decode the `k=v;k=v` body format;
- implement a custom `codec.PayloadCodec` for the example;
- expose helpers for building auth, heartbeat, telemetry, command, and ack messages.

This package exists to teach protocol authoring. It should stay small and intentionally example-scoped.

### `server` Program

Path: `examples/device-gateway/server`

Responsibilities:

- parse `-addr`;
- configure `server.NewServer`;
- use `packet.NewVariableLengthCodec()`;
- use the example device payload codec;
- wire `OnChannel`, `OnChannelClosed`, and `Router`;
- maintain device sessions and timeout scanning;
- send one example command after a device becomes active.

### `device` Program

Path: `examples/device-gateway/device`

Responsibilities:

- parse `-addr`, `-device-id`, and `-secret`;
- configure `client.NewClient`;
- connect to the gateway;
- send `auth`;
- start periodic `heartbeat`;
- send one sample `telemetry` report;
- receive one `command`;
- send `command_ack`;
- print inbound and outbound events in a readable way.

The device program should remain a simulator for one device session. It should not become a general-purpose test harness.

### Internal Server Components

The server should keep four internal components with narrow boundaries.

#### `registry`

Owns active sessions. Each session stores only:

- `deviceID`
- `channel`
- `authenticated`
- `connectedAt`
- `lastHeartbeatAt`

Responsibilities:

- register a new channel;
- bind `deviceID` after successful auth;
- update heartbeats;
- find sessions;
- remove sessions on close.

#### `router`

Dispatches decoded protocol messages by type:

- `auth -> authHandler`
- `heartbeat -> heartbeatHandler`
- `telemetry -> telemetryHandler`
- `command_ack -> commandAckHandler`

This should make the `less.Router` role obvious: route first, execute second.

#### `handlers`

Perform per-message business work:

- `authHandler` validates credentials, marks the session authenticated, and replies with `auth_ack`;
- `heartbeatHandler` updates liveness only for authenticated devices;
- `telemetryHandler` accepts telemetry only for authenticated devices and triggers one sample command write;
- `commandAckHandler` records the command acknowledgment.

#### `watchdog`

Runs in the background and closes sessions whose heartbeats are stale.

This component exists to demonstrate that server-side policy can close channels and that `OnChannelClosed` remains the one cleanup path.

## State Model

Each session should have exactly three states:

- `connected`
- `authenticated`
- `closed`

Transitions:

1. New accepted channel starts in `connected`.
2. Valid `auth` transitions the session to `authenticated`.
3. Any close path transitions the session to `closed`.

Allowed message rules:

- In `connected`, only `auth` is valid.
- In `authenticated`, `heartbeat`, `telemetry`, and `command_ack` are valid.
- In `closed`, nothing is processed.

Invalid messages should be handled explicitly. The implementation plan can decide whether the example prefers protocol error replies or immediate connection close for each case, but that behavior must be deterministic and tested.

## Data Flow

Recommended happy path:

1. Device dials the gateway.
2. `OnChannel` creates an unauthenticated session in the registry.
3. Device sends `auth`.
4. Payload codec decodes the binary-header-plus-KV message into a typed auth message.
5. `Router` selects `authHandler`.
6. `authHandler` validates the secret, binds the `deviceID`, marks the session authenticated, and writes `auth_ack`.
7. Device starts sending periodic `heartbeat`.
8. Device sends one `telemetry` message.
9. Server receives telemetry and writes one `command` back to the same channel.
10. Device decodes the command and replies with `command_ack` using the same `request_id`.
11. Device exits or the server eventually closes the connection.
12. `OnChannelClosed` removes the session from the registry.

## Error Handling

The example should be explicit about protocol errors.

At minimum cover these cases:

- unsupported `version`;
- unknown `type`;
- `body_length` mismatch;
- malformed key-value body;
- `heartbeat` before successful auth;
- `telemetry` before successful auth;
- duplicate auth on an already authenticated session;
- heartbeat timeout detected by the watchdog.

The implementation should prefer small, deterministic rules over feature-rich recovery. The point is to show where protocol validation lives, not to build a fault-tolerant device platform.

## Testing

Testing should prove both business behavior and protocol behavior.

### Protocol Unit Tests

Add focused tests for the `protocol` package:

- header encode/decode round-trip;
- typed message encode/decode round-trip;
- unknown type rejection;
- invalid version rejection;
- declared body length mismatch;
- malformed key-value body.

### End-to-End Integration Test

Add one integration test that uses public APIs and a dynamic TCP address to cover:

1. gateway starts successfully;
2. device client connects;
3. device sends `auth`;
4. device receives `auth_ack`;
5. device sends `heartbeat`;
6. device sends `telemetry`;
7. server sends `command`;
8. device sends `command_ack`;
9. connection closes;
10. server cleanup runs and the session disappears from the registry.

The integration test should avoid fixed sleeps where possible and should assert observable state or message delivery, not log lines.

## Documentation Expectations

The example should include a short `README` under `examples/device-gateway/` explaining:

- why this example keeps `PacketCodec` standard and customizes `PayloadCodec`;
- how the on-wire message format is structured;
- which files to edit to adapt the protocol for a real project;
- where `OnChannel`, `Router`, `Channel.Write`, and `OnChannelClosed` appear in the flow.

The README should frame the example as a protocol-authoring tutorial layered on a realistic gateway scenario.

## Implementation Boundaries

This design should be implementable without changing:

- `less.go`;
- `server/`;
- `client/`;
- `transport/`;
- `internal/channel`;
- `internal/engine`;
- existing public codec packages.

If the implementation exposes an ergonomics gap for custom codecs, that should be captured clearly in the implementation plan instead of being silently worked around.

## Acceptance Criteria

- `go run ./examples/device-gateway/server -addr 127.0.0.1:9000` starts the gateway.
- `go run ./examples/device-gateway/device -addr 127.0.0.1:9000 -device-id dev-001 -secret demo-secret` connects and authenticates.
- The device sends heartbeat and telemetry over the custom protocol.
- The server sends at least one command over the same connection.
- The device replies with `command_ack` correlated by `request_id`.
- Session cleanup runs through `OnChannelClosed`.
- Protocol unit tests and at least one end-to-end integration test pass.
- No third-party production dependency is added.
