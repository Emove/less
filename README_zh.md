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
- 可扩展替换网络协议和网络库（包括事件驱动型网络框架，如<a href="https://github.com/panjf2000/gnet">Gnet<a>、<a href="https://github.com/cloudwego/netpoll">Netpoll</a>等），默认实现了Go网络库的TCP协议
- 可扩展、自定义消息传输协议，并默认实现了常见的编解码器
- 可通过添加Middleware自定义读写中间件
- 基于Middleware的路由设计
- 零拷贝的编解码API设计
- 提供标准日志接口，可方便接入第三方Log库
- 内嵌Keepalive实现

## 安装
```shell
$ go get -u github.com/emove/less
```

## 快速开始
