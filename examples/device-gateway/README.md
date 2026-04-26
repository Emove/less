# Device Gateway Example

This example shows a complete `less` message flow around a small device gateway:

- device connects and authenticates
- server records the session in `OnChannel`
- device sends telemetry and heartbeat messages
- server routes telemetry to a handler and writes a command back
- device replies with a command ack
- server removes the session in `OnChannelClosed`

## Why the packet codec stays standard

The transport framing problem does not change for this example: each network payload still needs a stable length prefix. That is why both client and server keep `packet.NewVariableLengthCodec()`.

The example-specific part is the payload protocol. `examples/device-gateway/protocol` defines a custom `codec.PayloadCodec` that turns typed Go messages into a device-oriented wire payload and back. In other words:

- `PacketCodec`: generic transport envelope
- `PayloadCodec`: domain protocol

That split keeps the example focused on protocol teaching without changing the framework's transport layer.

## Wire format

Each payload is encoded as:

```text
| version:1 | type:1 | request_id:4 | body_length:4 | body:N |
```

- `version` is currently `1`
- `type` is one of auth, auth_ack, heartbeat, telemetry, command, command_ack
- `request_id` is used by command / command_ack
- `body` is an ASCII `key=value` list joined by `;`

Examples:

- auth body: `device_id=dev-001;secret=demo-secret`
- telemetry body: `battery=98;temp=23.4`
- command ack body: `status=ok`

The outer packet codec then prefixes that payload with the standard variable-length packet header.

## Read the files in this order

1. [`protocol/message.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/protocol/message.go): message types and protocol constants
2. [`protocol/codec.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/protocol/codec.go): custom payload encode/decode
3. [`server/handlers.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/server/handlers.go): channel hooks, router, and handlers
4. [`server/registry.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/server/registry.go): session state
5. [`server/main.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/server/main.go): server wiring
6. [`device/main.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/device/main.go): device simulator wiring
7. [`server/server_test.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/server/server_test.go): end-to-end flow assertions

## Where the framework touchpoints are

- `OnChannel`: [`server/handlers.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/server/handlers.go), `onChannel`
- `Router`: [`server/handlers.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/server/handlers.go), `newRouter`; and [`device/main.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/device/main.go), `deviceRouter`
- `Channel.Write`: server replies in `authHandler` and `telemetryHandler`; device writes auth/telemetry/heartbeat in [`device/main.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/device/main.go)
- `OnChannelClosed`: [`server/handlers.go`](/Users/emov/workspace/projects/personal/less/examples/device-gateway/server/handlers.go), `onChannelClosed`; device-side shutdown hook is in `newDeviceClient`

## Run it

Start the server:

```bash
go run ./examples/device-gateway/server -addr 127.0.0.1:9000
```

In another terminal, start a device:

```bash
go run ./examples/device-gateway/device -addr 127.0.0.1:9000 -device-id dev-001 -secret demo-secret
```

Run only the example tests:

```bash
go test ./examples/device-gateway/... -count=1
```

Run the full repository tests:

```bash
go test ./... -count=1
```
