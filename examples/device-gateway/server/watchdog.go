package main

import (
	"context"
	"errors"
	"time"
)

var errHeartbeatTimeout = errors.New("heartbeat timeout")

func startWatchdog(ctx context.Context, gw *gateway, interval time.Duration) {
	if ctx == nil || gw == nil || interval <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				closeStaleSessions(gw, now)
			}
		}
	}()
}

func closeStaleSessions(gw *gateway, now time.Time) {
	if gw == nil {
		return
	}

	for _, ch := range gw.registry.stale(now, gw.heartbeatTimeout) {
		ch.Close(errHeartbeatTimeout)
	}
}
