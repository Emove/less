# Less Framebuf Linkbuffer Optimization Design

## Context

`internal/engine/framebuf` currently provides the right high-level ownership shape for the Less byte path, but its implementation is still allocation-heavy and partly inconsistent with borrowed-slice semantics.

The current implementation copies in several hot paths:

- `NewFrame` copies every payload into a new slice.
- `Reader.Slice` copies from the reader into a retained frame.
- `Writer.WriteBinary` copies every payload into a segment.
- `Writer.Append` copies every segment from the source writer.
- `Reader.ensure` repeatedly allocates temporary buffers, reads into them, and appends into the reader buffer.

There is also a correctness hazard: `Reader.Next` returns a borrowed slice, then may immediately compact the backing array. That can overwrite or move data while the returned borrowed slice is still expected to be valid.

CloudWeGo netpoll's `linkbuffer` solves a harder version of this problem with linked nodes, pooled allocation, reference-counted nodes, read/write cursor separation, zero-copy slicing, explicit release, and mcache-backed byte reuse. Less should adopt the core ideas, but not the full public API or external dependency surface. The goal is still a thin, transparent framework with internal performance mechanics hidden behind small contracts.

## Decision

Rewrite `internal/engine/framebuf` as a simplified internal linkbuffer-style implementation.

The public and cross-package contracts stay unchanged:

```go
type Frame interface {
	Bytes() []byte
	Retain()
	Release()
}

type ReaderBuffer interface {
	Peek(n int) ([]byte, error)
	Next(n int) ([]byte, error)
	Skip(n int) error
	Slice(n int) (Frame, error)
	Len() int
	Release()
}

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

All node, pool, refcount, and span mechanics remain private to `internal/engine/framebuf`.

## Goals

- Fix borrowed-slice lifecycle ambiguity, especially the `Next` plus compaction hazard.
- Reduce avoidable allocations on reader, writer, frame, append, and fragmented input paths.
- Preserve the existing codec and engine contracts.
- Keep production dependencies at zero third-party packages.
- Make ownership and release behavior testable with unit, integration, race, and benchmark coverage.
- Use netpoll `linkbuffer` as a reference design, not as a direct port.

## Non-Goals

- Do not expose a public nocopy API.
- Do not import `github.com/bytedance/gopkg/lang/mcache`.
- Do not change `codec.Frame.Bytes() []byte` in this redesign.
- Do not promise that reader, writer, or frame objects are generally goroutine-safe.
- Do not guarantee writer retry after `FlushTo` failure in the first version.

## Architecture

`framebuf` will be split into four internal units:

- `allocator`: owns node pooling and byte block pooling.
- `node`: represents one readable or writable span over a block.
- `reader`: reads from `io.Reader` into a linked node chain and serves `Peek`, `Next`, `Skip`, and `Slice`.
- `writer`: assembles outbound nodes and flushes them to an `io.Writer`.

The codec and engine packages continue to depend only on `codec.Frame`, `codec.ReaderBuffer`, and `codec.WriterBuffer`.

## Allocator

The allocator uses only the standard library.

`nodePool` is a `sync.Pool` for node objects. A node must be fully reset before returning to the pool:

- clear links
- clear block/span references
- reset read/write offsets
- reset flags
- reset reference count

`blockPools` are size-bucketed `sync.Pool` instances for byte slices. Buckets use power-of-two capacities. Initial buckets should cover:

- `512B`
- `1K`
- `2K`
- `4K`
- `8K`
- `16K`
- `32K`
- `64K`

`minBlockSize` should start at `4K`, matching netpoll's practical default for linkbuffer nodes.

`largeThreshold` should start at `64K`. Blocks larger than this are unmanaged and are not returned to small-block pools, preventing rare large payloads from polluting common small allocations.

`sync.Pool` is only a performance cache. Correctness must never depend on pool retention. Ownership is enforced by node/block reference counts and flags.

## Node Model

A node holds:

- the backing bytes
- readable start and end offsets
- writable end offset when used as a writer node
- reference count
- ownership flag: pooled or unmanaged
- optional readonly/exposed flag
- next pointer

Owned nodes return their blocks to `blockPools` when their reference count reaches zero.

Unmanaged nodes wrap external or large byte slices and never return the backing bytes to a pool. They still use the same refcount path so release behavior stays uniform.

## Frame Model

`Frame` has two concrete forms:

- `singleFrame`: references one node span.
- `multiFrame`: references multiple node spans.

Creating a frame increments references for all nodes it holds. Releasing the frame decrements those references. Duplicate `Release` is safe and has no side effect after the first terminal release.

`Frame.Bytes()` must return a stable contiguous `[]byte` because the public interface requires it. For `singleFrame`, `Bytes()` can return the node span directly. For `multiFrame`, `Bytes()` lazily flattens spans into an owned pooled block on first call and caches that contiguous view until release.

The returned bytes have read-only semantics by contract. Go cannot enforce this, but callers must not mutate the slice. This must be documented and tested where mutation would corrupt shared blocks.

## Reader Semantics

`Reader` reads directly into writable space in tail nodes. It no longer reads into temporary slices and appends into one large buffer.

`Peek(n)` and `Next(n)` continue to return a contiguous `[]byte`, because existing packet codecs read headers and delimiters directly from returned slices.

If the requested range is inside one node, the returned slice is borrowed directly from that node. If the requested range crosses nodes, the reader copies into a scratch block and returns that scratch slice.

Borrowed slices from `Peek` and `Next` are valid only until the next reader operation:

- `Peek`
- `Next`
- `Skip`
- `Slice`
- `Release`

Borrowed slices must not cross goroutine boundaries or be stored outside the synchronous decode stack.

`Next(n)` consumes data by advancing offsets only. It must not compact or copy unread bytes after returning the borrowed slice.

`Slice(n)` consumes data and returns a retained frame. If the slice is contained in one node, it returns a `singleFrame`. If it crosses nodes, it returns a `multiFrame`. The reader releases its own references to fully consumed nodes, but frame-held nodes remain alive until frame release.

`Reader.Release()` releases all nodes and scratch blocks still owned by the reader. Already retained frames remain valid.

After release, reader operations return errors or empty state; they must not panic.

## Writer Semantics

`Malloc(n)` returns writable bytes from the current writable node when possible. If the current node has insufficient capacity, writer allocates a new node.

`WriteBinary(p)` always treats caller-provided bytes as caller-owned and mutable. It copies small payloads into pooled writer blocks. Payloads larger than `largeThreshold` are copied into owned unmanaged bytes, then wrapped as unmanaged readonly nodes. The first implementation must not zero-copy wrap caller-provided `[]byte`, because the writer cannot prove the caller will not mutate it before flush.

`WriteFrame(f)` uses retain semantics, not transfer semantics. The writer retains the frame or its internal spans for its own lifetime, and the caller remains responsible for its existing `Release`. This preserves the current engine pattern:

```go
payload, err := payloadCodec.Marshal(message)
defer payload.Release()
writer.WriteFrame(payload)
```

`Append(src)` transfers writer contents. On success, source writer becomes empty/released, and a later `src.Release()` is a safe no-op. On failure, source remains owned by the caller and destination must not retain partial source state.

`FlushTo(dst)` writes nodes in order and handles short writes. If `dst.Write` returns `n > 0` with an error, writer first advances the written offset, then returns the error.

After a `FlushTo` failure, writer only guarantees that `Release()` can reclaim resources. It does not guarantee retry or continued encoding.

After a successful `FlushTo`, writer must not write the same bytes again on a second flush. It may become empty or released.

`Writer.Release()` is idempotent.

## Error Handling

Reader errors follow normal blocking-reader expectations:

- If enough bytes are already available, decode operations can succeed even if a later read would return `io.EOF`.
- If `ensure(n)` cannot collect enough bytes, it returns the underlying error or `io.ErrUnexpectedEOF`.
- Already buffered bytes remain owned until consumed or release.

Writer errors follow shutdown-oriented semantics:

- short writes and write errors are returned to engine
- engine closes the channel or connection according to existing behavior
- writer cleanup remains deterministic through `Release`

## Compatibility With Existing Code

No codec or engine interface changes are required.

The following existing behavior must continue to work:

- variable-length packet headers read through `Next`
- delimiter scanning through repeated `Peek`
- payload codecs using `Frame.Bytes`
- engine deferring payload frame release after outbound encode
- writer append rejecting unsupported writer implementations

## Tests

Frame tests:

- single-node frame remains stable after reader release
- multi-node frame remains stable after reader release
- multi-node frame lazily flattens on `Bytes`
- multiple frames sharing one block release in different orders
- duplicate frame release does not double-return blocks

Reader tests:

- `Next` does not invalidate returned borrowed data by compacting
- `Peek` and `Next` return contiguous data across node boundaries
- borrowed slices are not treated as stable after the next reader operation
- `Slice` across two or more nodes returns a retained frame
- fragmented input is decoded without repeated temporary allocations
- EOF and unexpected EOF behavior is deterministic

Writer tests:

- `Malloc` grows nodes without corrupting previous spans
- `WriteFrame` retains frame data without stealing caller ownership
- `Append` success transfers source contents and invalidates source safely
- `Append` failure leaves source and destination ownership intact
- `FlushTo` handles one-byte writes across segments
- `FlushTo` handles `n > 0, err != nil`
- failed `FlushTo` followed by `Release` reclaims all resources

Allocator tests:

- small owned blocks return to the expected pool path
- large unmanaged blocks do not enter small-block pools
- node reset clears links and offsets
- duplicate release does not return the same block twice

Integration tests:

- existing packet codec tests continue passing
- existing payload codec tests continue passing
- server round trip continues passing
- delimiter and variable-length codecs pass with fragmented input

Race tests:

- concurrent `Frame.Retain` and `Frame.Release` must be race-safe through atomic or locked reference accounting
- reader and writer are not advertised as goroutine-safe and should not rely on broad locking

## Benchmarks

Add benchmarks under `internal/engine/framebuf` for:

- small payload, around `64B`
- medium payload, around `1K`
- large payload, `64K+`
- fragmented reader input
- delimiter scanning
- `WriteFrame`
- `Append`
- `FlushTo`

The first implementation does not set a hard performance threshold. It must report `allocs/op` and `B/op` so future work can compare against the current slice implementation and validate whether the linkbuffer rewrite actually improves the hot path.

## Implementation Order

1. Add tests and benchmarks that describe the new behavior.
2. Introduce allocator and node primitives behind package-private APIs.
3. Implement frame types and reference accounting.
4. Replace reader internals with linked nodes.
5. Replace writer internals with linked nodes.
6. Re-run codec and engine integration tests.
7. Run `go test ./...` and targeted benchmarks.

## Risks

- Refcount bugs can cause stale slices, premature block reuse, or leaks.
- Multi-frame lazy flatten changes when allocation happens, so benchmarks must cover both direct write and `Bytes()` paths.
- `WriteFrame` retain semantics are safer for current engine code but may copy or retain more than a pure transfer model.
- `sync.Pool` can hide leaks during short tests, so tests must assert logical release behavior without assuming pool determinism.
- Large unmanaged wrapping is only safe for bytes owned by `framebuf`. Caller-provided `WriteBinary` data must be copied before wrapping.

## Success Criteria

- `framebuf` no longer compacts or overwrites data behind a just-returned `Next` slice.
- `Slice` can retain data across reader release without copying on the single-node path.
- Multi-node frames are represented without immediate flattening.
- `WriteFrame` is compatible with caller-side `defer payload.Release()`.
- `Append` has explicit transfer semantics and safe failure behavior.
- `FlushTo` handles short writes and partial-write errors.
- `Release` is idempotent across frame, reader, and writer.
- The project keeps zero external production dependencies.
- Existing integration tests pass.
- New benchmarks record allocation behavior for future optimization decisions.
