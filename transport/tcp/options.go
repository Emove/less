package tcp

import (
	"time"

	"github.com/emove/less/log"
	trans "github.com/emove/less/transport"
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
	Network:         "tcp",
	Timeout:         time.Second * 5, // default connect timeout
	Keepalive:       true,
	KeepAlivePeriod: time.Minute,
	Linger:          -1,
	NoDelay:         true,
}

type Network string

const (
	TCP  = "tcp"
	TCP4 = "tcp4"
	TCP6 = "tcp6"
)

// WithNetwork sets tcp network, TCP, TCP4, TCP6 is allowed
func WithNetwork(network Network) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			switch network {
			case TCP, TCP4, TCP6:
				tcpOps.Network = string(network)
			default:
				tcpOps.Network = TCP
				log.Warnf("network %s not supported, apply tcp by default", network)
			}
		}
	}
}

// WithTimeout sets dial timeout, only works in client
func WithTimeout(d time.Duration) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.Timeout = d
		}
	}
}

// WithKeepalive sets tcp keepalive
func WithKeepalive(keepalive bool) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.Keepalive = keepalive
		}
	}
}

// WithKeepalivePeriod sets tcp keepalive period
func WithKeepalivePeriod(period time.Duration) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.KeepAlivePeriod = period
		}
	}
}

// WithLinger sets tcp linger
func WithLinger(linger int) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.Linger = linger
		}
	}
}

// WithNoDelay sets tcp no delay
func WithNoDelay(delay bool) trans.Option {
	return func(ops trans.Options) {
		if tcpOps, ok := ops.(TCPOptions); ok {
			tcpOps.NoDelay = delay
		}
	}
}
