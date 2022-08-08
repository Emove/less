package transport

import (
	"net"
	"time"
)

type Reader interface {

	// Next 返回接下来的 n 个字节
	Next(n int) (buf []byte, err error)

	// Peek 返回 Reader 中前 n 个字节
	Peek(n int) (buf []byte, err error)

	// Skip 跳过前 n 个字节
	Skip(n int) (err error)

	// Until 阻塞读取数据直到遇到第一个 delim 时返回
	Until(delim byte) (line []byte, err error)

	// Release 释放 Reader
	Release() (err error)
}

type Writer interface {

	// Write 写入 buf
	Write(buf []byte) (n int, err error)

	// Malloc 分配 n 个字节的空间
	Malloc(n int) (buf []byte)

	// MallocLength 返回已分配的空间长度
	MallocLength() (length int)

	// Flush 提交 Writer 中写入的数据
	Flush() (err error)

	// Release 释放 Writer
	Release()
}

// Connection 对原生 net.Conn 和 netpoll.Connection 的封装
type Connection interface {
	// Reader 返回 Reader 对象
	Reader() Reader

	// Writer 返回 Writer 对象
	Writer() Writer

	// Close 关闭链接
	Close() error

	// LocalAddr 返回本地监听地址
	LocalAddr() net.Addr

	// RemoteAddr 返回对端地址
	RemoteAddr() net.Addr

	// SetReadTimeout 设置读超时时间
	// 参数为零值时，将不设置读超时时间
	SetReadTimeout(t time.Duration) error

	// SetIdleTimeout 设置链接的空闲时间
	SetIdleTimeout(t time.Duration) error
}
