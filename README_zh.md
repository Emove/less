# Less

[![GoDoc][1]][2] [![Go Report Card][3]][4]

[1]: https://godoc.org/github.com/emove/less?status.svg
[2]: https://godoc.org/github.com/emove/less
[3]: https://goreportcard.com/badge/github.com/emove/less
[4]: https://goreportcard.com/report/github.com/emove/less

[English](README.md) | 中文

Less 是一个轻量、可组合的 Go 网络框架。当前仓库的公开使用面主要集中在：

- `less`：`Channel`、`Router`、`Handler`、`Middleware` 以及连接生命周期钩子
- `server`：服务端组装与启动
- `client`：客户端拨号与活动连接管理
- `codec`：编解码抽象，以及 `codec/packet`、`codec/payload` 下的内置实现
- `transport`：传输层抽象，默认 TCP 实现在 `transport/tcp`
- `log`：可替换的日志门面

当前仓库已经具备可工作的 server/client 主路径、内置编解码器、可运行的 chatroom 示例，以及覆盖核心生命周期的端到端测试。

## 当前状态

- 默认传输层为 `transport/tcp`
- 默认编解码组合为“变长包 + 文本负载”
- server 和 client 都支持 `OnChannel`、`OnChannelClosed`、中间件、路由、PacketCodec、PayloadCodec 配置

## 包结构总览

| 包 | 说明 |
| --- | --- |
| `less` | 核心接口与路由/中间件契约 |
| `server` | 构建并运行 Less 服务端 |
| `client` | 连接服务端并管理活动 Channel |
| `codec/packet` | 报文分帧策略，如变长包、定长包、分隔符包 |
| `codec/payload` | 负载序列化，如文本、JSON |
| `transport` | 框架使用的传输层抽象 |
| `transport/tcp` | 基于 Go `net` 的内置 TCP 实现 |
| `log` | 日志接口与默认日志器 |

## 内置编解码器

### PacketCodec

- `packet.NewVariableLengthCodec()`
- `packet.NewFixedLengthCodec(length)`
- `packet.NewDelimiterCodec(delimiter, maxLength, ...options)`

### PayloadCodec

- `payload.NewTextCodec()`
- `payload.NewJSONCodec()`
- `payload.NewJSONCodecWithType(sample)`

## 安装

```shell
go get github.com/emove/less
```

## 快速开始

最小可用服务端的组成是：选择传输层、选择编解码器、提供路由，然后启动服务。

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

说明：

- `server.Run()` 会异步启动传输层监听
- `WithRouter(...)` 是必需项
- 如果不显式覆盖编解码器，默认使用“变长包 + 文本负载”

## 运行示例

仓库自带一个可运行的聊天室示例，位于 `examples/chatroom`。

启动服务端：

```shell
go run ./examples/chatroom/server -addr 127.0.0.1:8888
```

在一个或多个新终端中启动客户端：

```shell
go run ./examples/chatroom/client -addr 127.0.0.1:8888
```

chatroom 示例使用了：

- `packet.NewVariableLengthCodec()` 做分帧
- `chat.NewJSONCodec()` 做消息序列化
- `OnChannel` / `OnChannelClosed` 处理加入与离开事件
- router 分发 `set_name`、`chat` 与系统消息

相关代码：

- `examples/chatroom/server`
- `examples/chatroom/client`
- `examples/chatroom/chat`

## 请求生命周期

当前请求路径是：

1. transport 接受连接或完成拨号
2. Channel 激活时执行 `OnChannel`
3. PacketCodec 将字节流解码为 frame
4. PayloadCodec 将 frame 反序列化为消息
5. 执行入站中间件
6. router 选择业务 handler
7. handler 执行业务逻辑
8. 写路径经过出站中间件、PayloadCodec 序列化、PacketCodec 编码与底层连接 flush
9. Channel 关闭时执行 `OnChannelClosed`

中间件执行顺序为：

- 入站：全局 inbound -> Channel inbound -> router
- 出站：Channel outbound -> 全局 outbound -> write handler

## 传输层模型

Less 使用统一的 `transport.Transport` 抽象：

- `Listen(addr, driver)`
- `Dial(network, addr, driver)`
- `Close()`

仓库当前只内置了 `transport/tcp`。如果需要其他后端，实现 `transport.Transport` 后通过 `WithTransport(...)` 注入即可。

## 测试

运行全部测试：

```shell
go test ./...
```

常用聚焦测试：

```shell
go test ./server ./client ./test/e2e
```

当前测试覆盖包括：

- server/client 生命周期
- 编解码链路读写
- Channel 生命周期钩子
- chatroom 示例行为
- 连接建立、消息收发、连接关闭的端到端流程

## 贡献说明

- `internal/...` 视为不稳定实现细节
- 对外示例优先使用 `less`、`server`、`client`、`codec`、`transport`、`log` 这些公共包
- 如果修改了对外行为，请同步更新 `README.md` 与 `README_zh.md`
