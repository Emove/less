# less Tier 1 End-to-End Reliability Test Design

## Context

`less` 当前已经具备真实 TCP server-client 链路：

- `server.NewServer(...).Run()` 通过 `transport.Listen` 接入 `internal/engine`。
- `client.NewClient(...).Dial(ctx)` 通过 `transport.Dial` 建立主动连接。
- 双方连接建立后都使用同一个 `less.Channel` 契约。
- 入站路径为 packet decode、payload unmarshal、middleware、router。
- 出站路径为 channel write、middleware、payload marshal、packet encode、transport write。

现有测试已经覆盖不少局部契约，例如 channel close 幂等、pipeline 顺序、server run fake transport、client dial 生命周期，以及一个真实 TCP 的双向消息测试。但还缺少一组默认进入 CI 的端到端可靠性门禁，用公共 API 一次性验证完整生命周期和关键失败路径。

## Goal

设计一组 Tier 1 端到端测试，默认由 `go test ./...` 执行，新增耗时控制在 30 秒以内。

这些测试必须从公共 API 进入，不直接依赖 `internal/engine` 或 fake transport：

- `server.NewServer`
- `client.NewClient`
- `transport/tcp`
- 真实 packet codec
- 真实 payload codec
- `less.Channel`

目标是让核心 server-client 行为在 CI 中持续可验证：连接能建立，消息能完整往返，关闭只通知一次，异常输入能收敛为可观察的连接关闭。

## Non-Goals

- 不做 Tier 2 stress 或 soak 测试。
- 不新增独立压测工具或脚本。
- 不引入第三方测试依赖。
- 不改变公开 API。
- 不把测试建立在 fixed port、固定 sleep 或日志观察上。
- 不追求性能 benchmark 数字。

## Test Architecture

新增或整理一个公共 API 视角的 e2e 测试文件，建议放在 `client` 包测试中，因为 client 已经是 server-client 真实链路的自然入口。

测试 harness 应该提供以下能力：

- 动态分配 `127.0.0.1:0` 端口，避免端口冲突。
- 启动 server 后使用 dial retry 等待监听就绪。
- 通过 channel、wait group、atomic counter 收集生命周期事件。
- 使用 deadline 包裹所有等待，避免 CI hang。
- 使用 `t.Cleanup` 统一关闭 client 和 server。
- 使用 stdlib 实现断言和同步。

测试 helper 应保持小而明确，不暴露新的框架抽象。建议 helper 只负责：

- reserve TCP address
- dial eventually
- wait until condition
- record lifecycle events
- build echo router
- build deterministic codec used by tests

## Scenarios

### 1. Full Lifecycle

覆盖单连接完整路径：

1. server `Run`
2. client `Dial`
3. server `OnChannel` fires once
4. client `OnChannel` fires once
5. client writes message to server
6. server router receives decoded message
7. server writes reply to client
8. client router receives decoded reply
9. client closes
10. server `OnChannelClosed` fires once
11. client `OnChannelClosed` fires once

断言重点：

- 双方 `Channel` 非 nil 且曾经 active。
- 两端消息内容完全匹配。
- close hook exactly-once。
- close 后 `client.Channel()` 变为 nil。

### 2. Concurrent Clients

覆盖多个真实 client 同时连接同一个 server。

建议规模：

- client 数量：8 到 16。
- 每个 client 消息数：5 到 10。
- 每条消息包含 client id 和 sequence。

server router 对每条消息 ack 或 echo。client 端断言收到自己对应的 ack。

断言重点：

- 总接收数等于 `clients * messagesPerClient`。
- 每个 client 收到自己的完整 ack 序列。
- 不出现跨 client 串包。
- 所有 client close 后，server close hook 数量等于 client 数量。

这个规模足以覆盖并发 channel、transport read loop、codec 边界和关闭路径，同时不会把 CI 时间拖长。

### 3. Server Shutdown With Active Clients

覆盖服务端主动 shutdown 时的活跃连接收敛。

流程：

1. 建立多个 client。
2. 确认 server 收到所有 `OnChannel`。
3. 不主动关闭 client，直接调用 `srv.Shutdown(ctx, err)`。
4. 等待 client 侧 `OnChannelClosed`。
5. 等待 server 侧 `OnChannelClosed`。

断言重点：

- 所有活跃连接最终关闭。
- 每个 close hook exactly-once。
- `client.Channel()` 最终为 nil。
- `srv.Shutdown` 不依赖业务消息继续流动。

### 4. Codec And Frame Boundaries

覆盖 packet/payload 边界而不进入 soak。

建议包含三类 payload：

- 小 payload，例如 `"ping"`。
- 连续多包，例如同一连接快速发送 10 条消息。
- 较大 payload，例如 32 KiB 到 128 KiB 的字符串。

断言重点：

- 所有 payload decode 后内容一致。
- 连续发送不会合并成一条业务消息。
- 大 payload 能完成双向收发。

该场景用于捕获 frame buffer、packet length、read loop 复用 reader 等边界问题。

### 5. Failure Path

覆盖异常输入的连接关闭行为。

推荐先用 raw `net.Dial` 连到 server，发送 malformed packet，例如声明长度大于实际 body 后直接关闭，或发送超过 `MaxReceiveMessageSize` 的 payload。

断言重点：

- server 不调用业务 handler，或只在符合当前实现语义时调用。
- server `OnChannelClosed` fires once。
- 连接最终关闭。
- 测试不依赖 panic 或日志。

如果当前实现对 oversized payload 是 decode/unmarshal 后丢弃而不关闭，测试应先锁住当前明确契约；若契约要求关闭，则实现计划中应包含对应修复。

## Error Handling And Synchronization

所有 e2e 测试必须避免不确定等待：

- 每个 wait 使用 `time.After` 或 context deadline。
- 不使用裸 `time.Sleep` 作为成功条件。
- 监听启动使用 dial retry，而不是假设 `Run()` 返回后 listener 已经 ready。
- close hook 使用 buffered channel 或 atomic counter，防止 goroutine 阻塞。
- cleanup 可以重复调用 `cli.Close` 和 `srv.Shutdown`，测试应验证幂等而不是依赖调用顺序。

失败时输出应包含 client id、message sequence、expected/got counters，便于定位并发失败。

## Runtime Budget

Tier 1 是 CI 门禁，不应成为压力测试。

建议预算：

- full lifecycle：1 到 2 秒内。
- concurrent clients：3 到 8 秒内。
- server shutdown：1 到 3 秒内。
- codec/frame boundary：1 到 3 秒内。
- failure path：1 到 3 秒内。

总新增耗时目标：正常机器上低于 15 秒，慢 CI 上低于 30 秒。

## Acceptance Criteria

设计完成后的实现应满足：

- `go test ./...` 默认执行 Tier 1 e2e 测试。
- 不新增生产依赖。
- 不新增测试第三方依赖，除非后续明确批准。
- 不使用固定端口。
- 不使用日志作为断言来源。
- 完整生命周期测试覆盖 server 和 client 双方 close hook exactly-once。
- 并发测试覆盖多个 client 的多消息双向收发。
- shutdown 测试覆盖 active clients 被 server shutdown 收敛。
- codec/frame boundary 测试覆盖小包、连续多包和较大 payload。
- failure path 测试覆盖 malformed 或 oversized input 的关闭行为。
- 整体新增测试耗时符合 CI `<=30s` 目标。

## Implementation Notes

实现计划应优先复用现有 `client/client_test.go` 中的真实 TCP 测试 helper，但需要把当前测试中隐含的行为改成明确断言：

- `reserveTCPAddr`
- `dialClientEventually`
- `waitFor`
- `messageFlowPacketCodec`
- `messageFlowPayloadCodec`

当前 `server/server_test.go` 中基于 `localhost:8888`、package-level wait group 和日志观察的测试可以保留或后续整理，但 Tier 1 e2e 门禁应避免继续依赖这种形态。

