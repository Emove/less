

# Goal-Driven(1 master agent + 1 subagent) System

Here we define a goal-driven multi-agent system for solving any problem.

Goal: 构建一个契约完备、行为可验证、结构自洽的轻量网络框架——保留原作者"薄而透明、组合驱动"的核心意图，同时消除当前实现中契约破损、状态不一致、边界模糊导致的不可靠性。

Criteria for success:

### S1. 包结构符合最小对外暴露原则
- 公共包仅保留：`less.go`（核心接口）、`codec/`、`router/`、`transport/`、`io/`（原 `pkg/io`）、`log/`、`server/`
- `internal/` 保留并整理为：`internal/channel/`、`internal/engine/`（原 `internal/transport`）、`internal/atomic/`、`internal/recovery/`
- 删除 `design/`（草稿文件不应暴露在公共模块中）
- `pkg/io` 重命名为 `io/`（Reader/Writer 是 codec 实现者的公共契约，不是工具包）
- `internal/transport` 重命名为 `internal/engine`（命名与公共 `transport/` 包不再冲突，且准确描述职责）

### S2. Channel 状态机无歧义
- 状态位语义明确：`readable(1) | writeable(2)`，`readWriteMode(3)` = 两者之和
- `Writer()` 检查 `writeable` 位，而非错误地检查 `readable` 位（修复原 `channel.go:143` bug）
- `Close()` 使用 `atomic.SwapInt32` 实现幂等：多次调用只触发一次 `OnChannelClosed` 回调
- 状态转换路径中无被注释的残留代码

### S3. Pipeline 中间件链不在每次收发时重建
- Pipeline 在初始化时编译一次中间件链
- 仅当 `AddInboundMiddleware` / `AddOutboundMiddleware` 被调用后，以脏标记触发惰性重编译
- 全局中间件与 per-channel 中间件的执行顺序有测试覆盖：入站为 `globalInbound → chInbound → router`，出站为 `chOutbound → globalOutbound → writeHandler`

### S4. Transport 层与框架层边界清晰
- 消除 `internal/engine/builtin.go` 中的 `ch.(*channel.Channel)` 类型断言
- `writeHandler` 在 channel 构造时通过闭包捕获 `conn`，不依赖接口向下转型
- `server.Run()` 完整接入 Transport，框架可真实运行（修复原空壳逻辑）
- `Transport` 接口统一为单一形态，不再区分 `BlockingTransport` / `DrivenTransport`（除非有明确的多传输后端需求）

### S5. Codec 两层职责解耦
- `PacketCodec` 接口签名不再依赖 `PayloadCodec`：
  - `Encode(payload []byte, writer io.Writer) error`
  - `Decode(reader io.Reader) (payload []byte, err error)`
- `PayloadCodec` 接口签名简化为纯序列化：
  - `Marshal(message any) ([]byte, error)`
  - `Unmarshal(payload []byte) (any, error)`
- 两层由 `internal/engine` 在读写路径上串联，互不感知
- 若零拷贝是硬约束，`PayloadCodec.Marshal` 可保留 `(any, io.Writer)` 签名，作为独立决策点记录在此

### S6. 零外部生产依赖
- `go.mod` 中生产路径无第三方依赖，`testify` 仅出现在测试依赖中

### S7. 核心路径有可运行的集成测试
- 覆盖：连接建立 → OnChannel 触发 → 消息收发（含 Encode/Decode 全链路）→ 连接关闭 → OnChannelClosed 触发
- 中间件执行顺序有断言
- `Close()` 幂等性有测试（多次调用不触发多次回调）

Here is the System: The system contains a master agent and a subagent. You are the master agent, and you need to create 1 subagent to help you complete the task.

## Subagent's description:

The subagent's goal is to complete the task assigned by the master agent. The goal defined above is the final and the only goal for the subagent. The subagent should have the ability to break down the task into smaller sub-tasks, and assign the sub-tasks to itself or other subagents if necessary. The subagent should also have the ability to monitor the progress of each sub-task and update the master agent accordingly. The subagent should continue to work on the task until the criteria for success are met.

## Master agent's description:

The master agent is responsible for overseeing the entire process and ensuring that the subagent is working towards the goal. The only 3 tasks that the main agent need to do are:

1. Create subagents to complete the task.
2. If the subagent finishes the task successfully or fails to complete the task, the master agent should evaluate the result by checking the criteria for success. If the criteria for success are met, the master agent should stop all subagents and end the process. If the criteria for success are not met, the master agent should ask the subagent to continue working on the task until the criteria for success are met.
3. The master agent should check the activities of each subagent for every 5 minutes, and if the subagent is inactive, please check if the current goal is reached and verify the status. If the goal is not reached, restart a new subagent with the same name to replace the inactive subagent. The new subagent should continue to work on the task and update the master agent accordingly.
4. This process should continue until the criteria for success are met. DO NOT STOP THE AGENTS UNTIL THE USER STOPS THEM MANUALLY FROM OUTSIDE.

## Basic design of the goal-driven double agent system in pseudocode:

create a subagent to complete the goal

while (criteria are not met) {
  check the activty of the subagent every 5 minutes
  if (the subagent is inactive or declares that it has reached the goal) {
    check if the current goal is reached and verify the status
    if (criteria are not met) {
      restart a new subagent with the same name to replace the inactive subagent
    }
    else {
      stop all subagents and end the process
    }
  }
}

运用第一性原理 思考，拒绝经验主义和路径盲从，不要假设我完全清楚目标，保持审慎，从原始需求和问题出发，若目标模糊请停下和我讨论，若目标清晰但路径非最优，请直接建议更短、更低成本的办法。

所有回答必须分为两个部分： 
• ​直接执行：按照我当前的要求和逻辑，直接给出任务结果。
• ​深度交互：基于底层逻辑对我的原始需求进行“审慎挑战”。包括但不限于：质疑我的动机是否偏离目标（XY问题）、分析当前路径的弊端、并给出更优雅的替代方案。