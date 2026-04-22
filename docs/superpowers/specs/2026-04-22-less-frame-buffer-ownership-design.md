# Less Frame Buffer Ownership Redesign

## Context

The current repository has already simplified major kernel boundaries, but the byte-path contract is still incoherent.

Today, the public `io` package exposes buffered reader and writer semantics such as `Peek`, `Next`, `Malloc`, `Flush`, and `Release`. Those capabilities are not generic I/O; they are packet-framing and buffer-lifecycle semantics. Because they are public, they leak transport and allocation strategy into framework contracts that should remain narrower.

The result is a three-layer coupling problem:

- `transport.Connection` exposes framework-specific buffered I/O.
- `codec.PacketCodec` depends on those transport-shaped contracts.
- `engine` cannot own buffer lifecycle cleanly because the lifecycle contract is spread across public APIs.

The project goal for this redesign is to keep the author's thin, transparent, composition-driven intent while making ownership explicit and behavior verifiable.

## Product Decision

This redesign adopts the following product constraints:

- Treat this change as an intentional breaking change.
- Replace public buffered I/O contracts with internal frame-buffer ownership contracts.
- Elevate zero-copy from hidden implementation detail to explicit internal ownership semantics.
- Keep the public surface small and honest even if that removes historical compatibility.
- Keep `engine` as the single owner of buffer construction, reuse, transfer, and release.

## Goal

Redefine the byte path around explicit `Frame` and `Buffer` ownership so that:

- packet framing remains efficient and composition-friendly
- transport no longer exposes framework-specific buffered contracts
- payload and packet codecs become orthogonal layers
- all borrowed data and retained data have explicit lifetime rules
- the runtime can test and enforce these rules through integration and edge-case tests

## Design Principles

- Prefer explicit ownership over borrowed slices with informal rules.
- Keep zero-copy semantics internal to the kernel, not part of the public transport story.
- Preserve composition points only where they carry business meaning: router, middleware, lifecycle hooks, packet codec, payload codec.
- Keep the API smaller than the implementation mechanism.
- Allow copying when required for correctness; optimize for low-copy, not ideological zero-copy.

## Summary Decision

The redesign adopts an explicit internal `Frame/Buffer` ownership model.

It does not merely move the existing public `io` package under `internal/`; that would hide the old abstraction without fixing its contract problems. It also does not attempt a full `netpoll`-style nocopy runtime with broad public API surface. Instead, it takes the minimum useful internal model:

- `Frame` represents retained byte ownership.
- `ReaderBuffer` represents borrowed inbound access plus optional promotion to retained ownership.
- `WriterBuffer` represents segmented outbound assembly and ownership transfer.
- `engine` is the only layer that manages the lifecycle of these objects.

## Target Architecture

### Public Boundary

The public surface after this redesign remains:

- `less`
- `codec`
- `router`
- `transport`
- `server`
- `log`

The public `io` package is removed.

### Internal Boundary

Add:

- `internal/engine/framebuf/`

Keep:

- `internal/engine/`
- `internal/channel/`
- `internal/atomic/`
- `internal/recovery/`

`internal/engine/framebuf` becomes the only package that defines buffer ownership mechanics. No public package may expose `Peek`, `Next`, `Malloc`, `Flush`, `Release`, or analogous low-level buffer lifecycle operations.

## Core Model

### Frame

`Frame` is a retained byte object with explicit ownership. A frame may represent:

- a direct view into one contiguous backing block
- a stable aggregation across multiple blocks

Callers do not depend on which representation is used internally.

Minimum contract:

```go
type Frame interface {
	Bytes() []byte
	Retain()
	Release()
}
```

Required semantics:

- `Bytes()` returns a stable readable view for the lifetime of the retained frame.
- `Retain()` increments ownership so the frame can outlive the current consumer scope.
- `Release()` decrements ownership and is required on every retained path.
- Implementations must defend against accidental duplicate release without corrupting shared state, but the normative contract remains retain/release pairing rather than unrestricted repeated release.

### ReaderBuffer

`ReaderBuffer` is the inbound parsing surface. It provides borrowed access by default and explicit promotion to retained ownership when needed.

Minimum contract:

```go
type ReaderBuffer interface {
	Peek(n int) ([]byte, error)
	Next(n int) ([]byte, error)
	Skip(n int) error
	Slice(n int) (Frame, error)
	Len() int
	Release()
}
```

Required semantics:

- `Peek` and `Next` return borrowed slices.
- Borrowed slices are valid only until the next lifecycle-ending release of the owning reader buffer.
- `Slice` returns a retained `Frame` suitable for transfer across layers or goroutines.
- `Len` reports readable bytes currently available to the reader.
- `Release` ends the current reader-buffer ownership epoch and invalidates all previously borrowed slices.

### WriterBuffer

`WriterBuffer` is the outbound assembly surface. It supports preallocation, frame transfer, and segmented append without forcing early flattening.

Minimum contract:

```go
type WriterBuffer interface {
	Malloc(n int) ([]byte, error)
	WriteBinary(p []byte) error
	WriteFrame(f Frame) error
	Append(src WriterBuffer) error
	Len() int
	FlushTo(dst io.Writer) error
	Release()
}
```

Required semantics:

- `Malloc` returns writable reserved capacity owned exclusively by the writer buffer until flush or release.
- `WriteBinary` writes caller-owned bytes into the outbound buffer; it may copy.
- `WriteFrame` transfers frame ownership into the writer path.
- `Append` transfers the outbound contents of another writer buffer into the destination writer buffer.
- `FlushTo` commits the current assembled output to the destination writer.
- `Release` returns all retained memory and invalidates unflushed writable regions.

The spec does not require a particular internal node structure, but it assumes a segmented or linked representation is permitted and encouraged.

## Ownership Rules

These rules are mandatory, not advisory.

### Borrowed Slice Rules

- `Peek` and `Next` return borrowed data only.
- Borrowed data must never cross goroutine boundaries.
- Borrowed data must never be retained past `ReaderBuffer.Release()`.
- Borrowed data must never be stored in channel context, middleware state, router state, or message objects.

### Retained Frame Rules

- Any data that must outlive the current decode scope must be promoted to `Frame`.
- Any data sent across goroutines must be represented as `Frame`.
- Every retained frame path must eventually call `Release`.
- Implementations may aggregate across multiple internal blocks as needed, but callers always interact with the frame as one retained object.

### Writer Ownership Transfer Rules

- `WriteFrame` transfers ownership of the provided frame into the writer path.
- `Append` transfers ownership of the source writer contents into the destination writer.
- After ownership transfer, callers must not mutate, reuse, or release the transferred resource unless the interface explicitly states retain semantics.
- `FlushTo` failure leaves the destination connection in error-handling shutdown flow; partially written internal state is not guaranteed to remain reusable.

## Public Contract Changes

### Remove Public `io`

Delete the public `io/` package entirely.

This is a deliberate contract correction. The current package is not generic I/O and should not remain part of the public module surface.

### Redefine `transport.Connection`

`transport.Connection` must shrink back to connection semantics:

```go
type Connection interface {
	Read(buf []byte) (int, error)
	Write(buf []byte) (int, error)
	IsActive() bool
	Close() error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}
```

The following methods are removed:

- `Reader()`
- `Writer()`

Transport no longer knows about frame buffers or codec-facing buffer operations.

### Redefine `codec.PacketCodec`

Packet codec becomes frame-oriented rather than slice-oriented:

```go
type PacketCodec interface {
	Name() string
	Decode(src framebuf.ReaderBuffer) (framebuf.Frame, error)
	Encode(dst framebuf.WriterBuffer, payload framebuf.Frame) error
}
```

Consequences:

- Packet decoding may return a retained frame without flattening into a new `[]byte`.
- Packet encoding may write segmented output without first materializing a full packet slice.
- Packet codec remains responsible only for packet framing, not message serialization.

### Redefine `codec.PayloadCodec`

Payload codec also becomes frame-oriented:

```go
type PayloadCodec interface {
	Name() string
	Marshal(message any) (framebuf.Frame, error)
	Unmarshal(payload framebuf.Frame) (any, error)
}
```

Consequences:

- Payload codec consumes and produces retained byte ownership rather than raw slices.
- Payload codec implementations may still copy internally, but the contract no longer assumes that they must.
- Packet and payload layers remain decoupled and communicate only through retained frames.

## Engine Responsibilities

`internal/engine` becomes the sole orchestrator of byte-path ownership.

### Inbound Path

Inbound flow becomes:

1. `transport.Connection` delivers bytes through `Read`.
2. `engine` fills or advances a `ReaderBuffer`.
3. `PacketCodec.Decode` consumes from `ReaderBuffer` and returns a retained `Frame`.
4. `PayloadCodec.Unmarshal` consumes that frame and returns a message.
5. `engine` releases the payload frame after unmarshal returns.
6. Inbound middleware, router, and handler proceed on the message value.

### Outbound Path

Outbound flow becomes:

1. Handler or middleware calls `Channel.Write`.
2. Outbound middleware runs.
3. `PayloadCodec.Marshal` returns a retained payload frame.
4. `PacketCodec.Encode` writes packet structure into `WriterBuffer`, using frame transfer where appropriate.
5. `engine` flushes the writer buffer to `transport.Connection`.
6. `engine` releases payload and writer resources regardless of success or failure.

### Buffer Ownership

`engine` owns:

- creation of per-connection inbound and outbound buffers
- all release behavior for normal request paths
- recovery-path cleanup after decode, encode, or flush failure
- any pooling or reuse strategy hidden behind `framebuf`

No other package may assume responsibility for reusable byte-block pooling.

## Data Flow Guarantees

The redesign preserves the kernel flow while changing the byte contract:

`conn -> reader buffer -> packet decode -> payload decode -> inbound middleware -> router -> handler -> outbound middleware -> payload encode -> packet encode -> writer buffer -> conn`

Guarantees:

- packet and payload codecs remain separate layers
- router and middleware never see borrowed frame-buffer slices
- transport does not expose frame-buffer mechanics
- a frame returned by codec is stable for its retained lifetime

## Error Handling

Error handling must follow ownership cleanup rules exactly.

### Inbound Errors

- If `PacketCodec.Decode` returns an error, `engine` closes the channel/connection and releases reader-buffer scoped borrowed state.
- If `PayloadCodec.Unmarshal` returns an error, `engine` releases the returned frame and closes the channel/connection.
- If middleware, router, or handler returns an error, behavior remains whatever the existing kernel contract defines, but no borrowed buffer state may escape that path.

### Outbound Errors

- If `PayloadCodec.Marshal` returns an error, no writer work begins.
- If `PacketCodec.Encode` returns an error, `engine` releases payload and writer resources and fails the write path.
- If `FlushTo` returns an error, the connection enters shutdown flow and the current writer state is discarded.

### Panic Recovery

Panic recovery remains an internal concern. Recovery code must ensure:

- all owned frames are released
- writer buffers are released or discarded
- the channel transitions to closure once

## Implementation Constraints

To keep the redesign aligned with the repository goal, the implementation must not:

- reintroduce a public buffer-management package
- make transport responsible for packet or payload framing concerns
- require router or middleware authors to reason about frame ownership
- optimize around hypothetical future transports at the expense of current kernel clarity

The implementation may:

- use linked blocks, rings, or segmented slices internally
- copy when a retained stable frame must be materialized from discontiguous data
- add internal helper types to express pooled ownership safely

## Migration Scope

This redesign is a single breaking migration and must be implemented as one coherent contract shift.

Required migration targets:

- remove `io/`
- update all packet codec implementations to the new `Frame/Buffer` contract
- update payload codec implementations to the new `Frame` contract
- remove `Reader()` and `Writer()` from `transport.Connection`
- rework `transport/tcp` so it no longer constructs public buffered wrappers
- update `internal/channel` and `internal/engine` to stop exposing transport-buffer concepts
- update tests, mocks, and integration paths to reflect the new ownership model

The migration is not considered complete if adapters temporarily preserve the old public `io` surface.

## Testing Strategy

Testing must prove both behavior and ownership discipline.

### Contract Tests

- `PacketCodec` implementations preserve framing behavior after the contract migration.
- `PayloadCodec` implementations still round-trip messages correctly after the contract migration.
- `transport.Connection` no longer exposes `Reader()` or `Writer()`.

### Ownership Tests

- borrowed `Peek` and `Next` slices become invalid across release boundaries by contract
- `Slice` returns a stable retained frame even when backed by discontiguous input
- accidental duplicate `Frame.Release()` does not corrupt shared state
- `WriteFrame` and `Append` obey ownership transfer semantics consistently

### Behavioral Tests

- full connection lifecycle still works: connect, `OnChannel`, inbound decode, handler, outbound encode, close, `OnChannelClosed`
- middleware order remains unchanged
- write failures and decode failures trigger deterministic cleanup and closure
- shutdown still closes active connections and listener state cleanly

### Performance-Shape Tests

The spec does not set hard benchmark thresholds, but tests should demonstrate:

- delimiter-style packet decoding can succeed without mandatory payload copy when the delimiter and body are available contiguously
- segmented outbound assembly does not require pre-flattening into one packet slice before flush

## Success Criteria

The redesign succeeds when all of the following are true:

- the public `io` package is deleted
- public transport contracts no longer expose buffer lifecycle operations
- packet and payload codec contracts are both frame-oriented
- `engine` is the only layer that owns frame-buffer lifecycle management
- lifecycle and ownership rules are documented and covered by tests
- the kernel remains thin: middleware, router, hooks, and handlers do not need to reason about internal frame-buffer mechanics

## Non-Goals

This redesign does not aim to:

- provide a public nocopy API for user code
- guarantee absolute zero-copy on every path
- implement a generalized multi-transport optimization layer
- preserve import compatibility with old packet or payload codec contracts
- expose internal frame-buffer node structure as a supported extension surface

## Recommended Implementation Order

The implementation plan that follows this spec should proceed in this order:

1. define `internal/engine/framebuf` contracts and ownership rules
2. migrate packet codec contracts and implementations
3. migrate payload codec contracts and implementations
4. shrink `transport.Connection` back to raw connection semantics
5. rewire `engine` read and write paths around internal buffer ownership
6. remove the public `io` package and obsolete wrappers
7. update tests and integration coverage for ownership and lifecycle behavior
