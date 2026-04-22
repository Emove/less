# Less Kernel Simplification Design

## Context

The current repository has already completed a first structural cleanup pass, but the project still presents itself as a broader network framework than the implementation can reliably support. The stated intent is a thin, transparent, composition-driven networking core. The remaining problems are not primarily naming or directory issues; they are mismatches between public promises, runtime behavior, and verification.

The highest-value cleanup direction is therefore an aggressive contraction into a small, honest kernel with explicit lifecycle semantics, minimal public surface, and behavior that can be fully tested.

## Product Decision

This redesign adopts the following constraints:

- Position the project as a transport framework kernel, not a general network platform.
- Allow breaking changes freely when they improve coherence.
- Keep lifecycle hooks as first-class kernel extension points.
- Treat zero-copy as an optimization goal, not as a hard contract that shapes the core API.

## Goal

Redefine `less` as a small message-transport kernel centered on one stable processing path:

`conn -> packet decode -> payload decode -> inbound middleware -> router -> handler -> outbound middleware -> payload encode -> packet encode -> conn`

Everything kept in the public surface must directly serve this path or the lifecycle boundaries around it.

## Design Principles

- Prefer explicit ownership over mirrored local state.
- Prefer fewer public contracts with stronger guarantees.
- Remove future-facing abstractions that are not part of the stable kernel.
- Keep extension at composition points only: lifecycle hooks, middleware, router, packet codec, payload codec.
- Make runtime facts and documented guarantees match exactly.

## Target Architecture

### Kernel Boundary

The kernel is an opinionated runtime for connection-oriented message handling. It is not a platform for many backends, a generic networking toolkit, or a staging area for future client features.

The runtime owns one connection lifecycle and one message pipeline:

- `Transport` provides real listener and connection events.
- `Engine` translates those events into framework behavior.
- `Channel` is the framework view of a live connection.
- `Codec` transforms between bytes and messages.
- `Router` chooses the business handler for inbound messages.
- Middleware composes cross-cutting behavior around inbound and outbound flow.

### Lifecycle Model

There must be one trusted lifecycle fact source: the transport-layer listener and connections. Framework state may reflect that fact, but must not invent a parallel closure model.

The normalized lifecycle rules are:

- `Server.Shutdown` closes the listener and drives all active connections toward closure.
- `Channel.Close(err)` requests closure of that connection and must correspond to real connection shutdown, not a local state-only transition.
- `Transport.Close()` must terminate listening and active I/O, not merely flip an internal flag.
- `OnChannel` fires after the framework creates and activates a channel for a newly accepted connection. Returning an error rejects the connection.
- `OnChannelClosed` fires exactly once after closure has become a real fact for that connection.

Hooks remain part of the stable kernel because they preserve the author's original intent: a thin core with well-defined business insertion points.

## Public Surface

### Keep

- `less`
  - `Channel`
  - `Handler`
  - `Middleware`
  - `OnChannel`
  - `OnChannelClosed`
- `codec`
  - packet and payload codec interfaces
  - default codec implementations
- `router`
  - router contract
- `transport`
  - minimal transport and connection contracts
- `server`
  - the main assembly entry point
- `io`
  - shared reader/writer contracts used by codecs

### Remove or Redefine

- Remove `Channel.CloseReader()`
- Remove `Channel.CloseWriter()`
- Remove `Channel.Readable()`
- Remove `Channel.Writeable()`

These APIs currently imply half-close semantics that the transport layer does not actually guarantee. They are contract inflation and should not remain in a minimal kernel.

- Make `router` a required construction dependency rather than an optional runtime footgun.
- Keep a single `Transport` abstraction instead of multiple transport styles.
- Keep lifecycle hooks, but narrow them to observation and admission boundaries rather than auxiliary shutdown control.

## Package Structure

### Public Packages

- `less`
- `codec`
- `router`
- `transport`
- `server`
- `io`

### Internal Packages

- `internal/channel`
- `internal/engine`
- `internal/atomic`
- `internal/recovery`

No additional public package should exist unless it directly contributes to the stable kernel path.

## Deletion and Contraction Scope

The redesign should explicitly delete or internalize:

- pseudo half-close behavior on `Channel`
- client and dialer narratives that are not part of the kernel target
- `GracefulCloser`-style abstractions that do not participate in the stable runtime path
- commented-out timeout/context design fragments
- shared mutable defaults across `server`, `engine`, and `transport/tcp`
- global pipeline pooling that breaks instance isolation
- README claims that exceed the kernel's real supported behavior

The project should stop presenting the following as part of its official identity unless they are reintroduced as complete, tested kernel features:

- multi-backend transport framework story
- keepalive integration story
- client roadmap as a current surface commitment
- broad configurability claims without matching public assembly points

## Data Flow

Inbound path:

1. `Transport` accepts a connection.
2. `Engine` constructs a `Channel`.
3. `OnChannel` hooks run.
4. Packet codec decodes frame payload.
5. Payload codec unmarshals message.
6. Inbound middleware executes.
7. Router resolves a handler.
8. Handler processes the message.

Outbound path:

1. Handler or middleware calls `Channel.Write`.
2. Outbound middleware executes.
3. Payload codec marshals the message.
4. Packet codec encodes the payload.
5. Transport writer flushes bytes to the connection.

Close path:

1. Closure is requested by peer disconnect, server shutdown, or `Channel.Close`.
2. Transport performs actual connection shutdown.
3. Channel becomes inactive in response to the real close event.
4. `OnChannelClosed` fires exactly once.

## Error Handling

- Connection admission errors reject the connection before it becomes active.
- Decode, unmarshal, routing, and handler failures close the affected connection unless explicitly handled by middleware.
- Panic recovery remains an internal boundary concern, not a public extension surface.
- Shutdown errors are reported through shutdown hooks or returned errors, but must not leave listener or connections partially alive.

## Testing Strategy

The redesign should be verified by behavior, not by package shape alone.

Required verification areas:

- listener startup and shutdown
- connection accept and rejection via `OnChannel`
- full inbound and outbound message flow through packet codec and payload codec
- exact middleware ordering
- exact `OnChannelClosed` once-only behavior
- configuration isolation between multiple independently constructed servers/handlers/transports
- deterministic shutdown that closes listeners and active connections

Tests must avoid shared global ports, shared global logger assumptions, and any cross-test hidden state.

## Non-Goals

- Supporting many transport backends as a first-class story
- Providing a public client subsystem in this redesign
- Encoding zero-copy behavior into the core public contracts
- Preserving old import compatibility when it conflicts with kernel clarity

## Migration Impact

This redesign is intentionally breaking.

Expected breaking changes include:

- reduced `Channel` interface
- stricter `server` construction requirements
- removal of unused or misleading transport/client abstractions
- rewritten README and examples to match the new kernel scope

The project should prefer a smaller honest API over compatibility with historical but incoherent behavior.

## Open Decisions Resolved

- Kernel identity: transport framework kernel
- Compatibility stance: breaking changes allowed
- Lifecycle hooks: preserved
- Zero-copy: retained as an optimization goal only

## Implementation Shape

The implementation plan that follows this spec should proceed in this order:

1. unify lifecycle ownership and close semantics
2. remove inflated public API and require explicit router construction
3. isolate configuration state per instance
4. remove stale abstractions and documentation drift
5. stabilize integration and contract tests around the new kernel
