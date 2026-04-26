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
- server 和 client 两侧都必须提供 `WithRouter(...)`
- client 在已有活动 session 时不允许重复 `Dial(...)`

## 示例怎么选

- `examples/chatroom`：适合先看最小可运行链路，包含 JSON 消息、广播和基本 hook/router 用法
- `examples/device-gateway`：适合看自定义业务 PayloadCodec、认证、心跳、遥测、命令/应答这类更接近真实协议的场景

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

## Client 快速开始

客户端的组装方式与服务端对称：选择编解码器、提供处理入站消息的 router，建立连接，然后使用活动 Channel。

```go
package main

import (
	"context"
	"fmt"

	"github.com/emove/less"
	"github.com/emove/less/client"
	"github.com/emove/less/codec/packet"
	"github.com/emove/less/codec/payload"
)

func main() {
	cli := client.NewClient(
		"tcp",
		"127.0.0.1:8080",
		client.WithPacketCodec(packet.NewVariableLengthCodec()),
		client.WithPayloadCodec(payload.NewTextCodec()),
		client.WithRouter(func(ctx context.Context, ch less.Channel, msg interface{}) (less.Handler, error) {
			return func(_ context.Context, _ less.Channel, msg interface{}) error {
				fmt.Println("recv:", msg.(string))
				return nil
			}, nil
		}),
	)
	defer cli.Close(nil)

	if err := cli.Dial(context.Background()); err != nil {
		panic(err)
	}

	ch := cli.Channel()
	if ch == nil {
		panic("dial succeeded without an active channel")
	}

	if err := ch.Write("hello"); err != nil {
		panic(err)
	}
}
```

说明：

- `client.Channel()` 在 `Dial(...)` 成功后返回当前活动 Channel
- `client.WithRouter(...)` 依然是必需项，因为客户端入站消息也走同一条 decode -> route -> handler 链路
- 如果当前 session 仍然活跃，重复拨号会直接返回错误

## 端点配置项

### Server 侧

- `server.WithTransport(...)`
- `server.WithPacketCodec(...)`
- `server.WithPayloadCodec(...)`
- `server.WithRouter(...)`
- `server.WithOnChannel(...)`
- `server.WithOnChannelClosed(...)`
- `server.WithInboundMiddleware(...)`
- `server.WithOutboundMiddleware(...)`
- `server.WithShutdownHooks(...)`
- `server.MaxChannelSize(...)`
- `server.MaxSendMessageSize(...)`
- `server.MaxReceiveMessageSize(...)`

### Client 侧

- `client.WithTransport(...)`
- `client.WithPacketCodec(...)`
- `client.WithPayloadCodec(...)`
- `client.WithRouter(...)`
- `client.WithOnChannel(...)`
- `client.WithOnChannelClosed(...)`
- `client.WithInboundMiddleware(...)`
- `client.WithOutboundMiddleware(...)`
- `client.MaxSendMessageSize(...)`
- `client.MaxReceiveMessageSize(...)`

## 运行示例

仓库当前自带两个可运行示例。

### Chatroom

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

### Device gateway

启动网关服务端：

```shell
go run ./examples/device-gateway/server -addr 127.0.0.1:9000
```

启动一个模拟设备：

```shell
go run ./examples/device-gateway/device -addr 127.0.0.1:9000 -device-id dev-001 -secret demo-secret
```

device-gateway 示例使用了：

- 标准变长包 codec 作为外层分帧
- `examples/device-gateway/protocol` 下的自定义 PayloadCodec
- `OnChannel` / `OnChannelClosed` 管理设备 session 的注册与清理
- 带有认证、心跳、遥测、命令、命令应答的类型化协议

相关代码：

- `examples/device-gateway/server`
- `examples/device-gateway/device`
- `examples/device-gateway/protocol`
- `examples/device-gateway/README.md`，用于阅读协议层 walkthrough

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

### 内置 TCP transport 配置

默认传输层由 `tcp.New()` 创建。要自定义参数，可以通过 `server.WithTransport(tcp.New(...))` 或 `client.WithTransport(tcp.New(...))` 注入。

当前支持的 TCP 选项包括：

- `tcp.WithNetwork(tcp.TCP | tcp.TCP4 | tcp.TCP6)`
- `tcp.WithTimeout(duration)`，用于客户端拨号超时
- `tcp.WithKeepalive(bool)`
- `tcp.WithKeepalivePeriod(duration)`
- `tcp.WithLinger(int)`
- `tcp.WithNoDelay(bool)`
