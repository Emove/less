# Less

[![GoDoc][1]][2] [![Go Report Card][3]][4]

<!--[![Downloads][7]][8]-->

[1]: https://godoc.org/github.com/emove/less?status.svg

[2]: https://godoc.org/github.com/emove/less

[3]: https://goreportcard.com/badge/github.com/emove/less

[4]: https://goreportcard.com/report/github.com/emove/less

English | [中文](README_zh.md)

## Introduction

Less is a ***light-weight*** and ***strong-extensibility*** network framework for Golang

### Feature

- Supports use different network library as transport layer（including Non-Blocking I/O network library，such as <a href="https://github.com/panjf2000/gnet">Gnet<a>
  、<a href="https://github.com/cloudwego/netpoll">Netpoll</a>），and provides Golang TCP network by default
- Provides common codec and supports customize message codec
- Pipelined Middleware
- flexible Router design
- Provides Non-Copy API for reading and writing
- Provides standard Logger interface
- Integrates keepalive implementation

### TODO

- [ ] Client

## Install

```shell
$ go get -u github.com/emove/less
```

## Quick Start

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

### More-Example
- [less-example](https://github.com/emove/less-example)