package keepalive

import "time"

type ClientParameters struct {
	Time    time.Duration
	Timeout time.Duration
}

type ServerParameters struct {
	MaxConnectionIdleTime time.Duration
	MaxConnectionAge      time.Duration
	Timeout               time.Duration
}
