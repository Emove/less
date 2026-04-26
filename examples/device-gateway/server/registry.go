package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/emove/less"
)

const demoSecret = "demo-secret"

type session struct {
	deviceID                string
	channel                 less.Channel
	authenticated           bool
	connectedAt             time.Time
	lastHeartbeatAt         time.Time
	lastCommandAckRequestID uint32
	lastCommandAckAt        time.Time
}

type registry struct {
	mu       sync.RWMutex
	sessions map[less.Channel]*session
}

type gateway struct {
	registry         *registry
	heartbeatTimeout time.Duration
	commandSeq       atomic.Uint32
}

func newGateway(timeout time.Duration) *gateway {
	return &gateway{
		registry: &registry{
			sessions: make(map[less.Channel]*session),
		},
		heartbeatTimeout: timeout,
	}
}

func (r *registry) register(ch less.Channel) {
	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[ch] = &session{
		channel:         ch,
		connectedAt:     now,
		lastHeartbeatAt: now,
	}
}

func (r *registry) remove(ch less.Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessions, ch)
}

func (r *registry) session(ch less.Channel) (*session, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sess, ok := r.sessions[ch]
	if !ok {
		return nil, false
	}

	cp := *sess
	return &cp, true
}

func (r *registry) authenticate(ch less.Channel, deviceID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	sess, ok := r.sessions[ch]
	if !ok {
		return false
	}

	sess.deviceID = deviceID
	sess.authenticated = true
	sess.lastHeartbeatAt = time.Now()
	return true
}

func (r *registry) heartbeat(ch less.Channel, at time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	sess, ok := r.sessions[ch]
	if !ok || !sess.authenticated {
		return false
	}

	sess.lastHeartbeatAt = at
	return true
}

func (r *registry) observeCommandAck(ch less.Channel, requestID uint32, at time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	sess, ok := r.sessions[ch]
	if !ok || !sess.authenticated {
		return false
	}

	sess.lastCommandAckRequestID = requestID
	sess.lastCommandAckAt = at
	return true
}

func (r *registry) stale(now time.Time, timeout time.Duration) []less.Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var channels []less.Channel
	for ch, sess := range r.sessions {
		if !sess.authenticated {
			continue
		}
		if now.Sub(sess.lastHeartbeatAt) > timeout {
			channels = append(channels, ch)
		}
	}

	return channels
}
