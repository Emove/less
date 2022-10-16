# Less

[![GoDoc][1]][2] [![Go Report Card][3]][4]

<!--[![Downloads][7]][8]-->

[1]: https://godoc.org/github.com/emove/less?status.svg

[2]: https://godoc.org/github.com/emove/less

[3]: https://goreportcard.com/badge/github.com/emove/less

[4]: https://goreportcard.com/report/github.com/emove/less

## 介绍

Less是一个简单易用的、灵活可扩展的轻量级网络中间件

### 特性

- 可扩展替换网络协议和网络库（包括事件驱动型网络框架，如<a href="https://github.com/panjf2000/gnet">Gnet<a>
  、<a href="https://github.com/cloudwego/netpoll">Netpoll</a>等），默认实现了Go网络库的TCP协议
- 可扩展、自定义消息传输协议，并默认实现了常见的编解码器
- 可通过添加Middleware自定义读写中间件
- 基于Middleware的路由设计
- 零拷贝的编解码API设计
- 提供标准日志接口，可方便接入第三方Log库
- 内嵌Keepalive实现

### TODO
- Client

## 安装


```shell
$ go get -u github.com/emove/less
```

## 快速开始

```go
package main

import (
	"context"
	"fmt"

	"github.com/emove/less"
	"github.com/emove/less/server"
)

func main() {
	// creates a less server
	// adds OnChannel hook and OnChannelClosed hook
	// adds a router
	srv := server.NewServer(":8080",
		server.WithOnChannel(OnChannelHook),
		server.WithOnChannelClosed(OnChannelClosedHook),
		server.WithRouter(router))

	// serving the network
	srv.Run()

	select {}
}

var IDGenerator uint32

type channelCtxKey struct{}

// ChannelContext custom channel context, used to identify channel
type ChannelContext struct {
	ID uint32
	Ch less.Channel
}

// OnChannelHook identifies each channel and print it
func OnChannelHook(ctx context.Context, ch less.Channel) (context.Context, error) {
	IDGenerator++
	fmt.Printf("new channel, id: %d, remote addr: %s\n", IDGenerator, ch.RemoteAddr().String())
	return context.WithValue(ctx, &channelCtxKey{}, &ChannelContext{ID: IDGenerator, Ch: ch}), nil
}

// OnChannelClosedHook prints channel id when channel closed
func OnChannelClosedHook(ctx context.Context, ch less.Channel, err error) {
	cc := ctx.Value(&channelCtxKey{}).(*ChannelContext)
	fmt.Printf("channel closed, id: %d, remote addr: %s ", cc.ID, ch.RemoteAddr().String())
	if err != nil {
		fmt.Printf("due to err: %v", err)
	}
	fmt.Println()
}

// router returns a handler to handle inbound message, it always return echoHandler
func router(ctx context.Context, channel less.Channel, msg interface{}) (less.Handler, error) {
	return echoHandler, nil
}

// echoHandler logic handler
func echoHandler(ctx context.Context, ch less.Channel, msg interface{}) error {
	cc := ctx.Value(&channelCtxKey{}).(*ChannelContext)
	fmt.Printf("receive msg from channel, id: %d, remote: %s, msg: %v\n", cc.ID, ch.RemoteAddr().String(), msg)
	return nil
}
```

### 更多示例
- [less-example](https://github.com/emove/less-example)