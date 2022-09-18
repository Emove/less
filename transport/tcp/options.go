package tcp

import (
	trans "github.com/emove/less/transport"
	"time"
)

type TCPOptions struct {
	Network         string
	Timeout         time.Duration
	Keepalive       bool
	KeepAlivePeriod time.Duration
	Linger          int
	NoDelay         bool
}

var DefaultOptions = &TCPOptions{
	Network: "tcp",
	// default connect timeout
	Timeout:         time.Second * 5,
	Keepalive:       true,
	KeepAlivePeriod: time.Minute,
	Linger:          -1,
	NoDelay:         true,
}

func WithNetwork(network string) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			switch network {
			case "tcp", "tcp4", "tcp6":
				tcpOps.Network = network
			}
		}
	}
}

func WithTimeout(d time.Duration) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.Timeout = d
		}
	}
}

func WithKeepalive(keepalive bool) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.Keepalive = keepalive
		}
	}
}

func WithKeepalivePeriod(period time.Duration) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.KeepAlivePeriod = period
		}
	}
}

func WithLinger(linger int) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.Linger = linger
		}
	}
}

func WithNoDelay(delay bool) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.NoDelay = delay
		}
	}
}
