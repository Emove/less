package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/emove/less"
	"github.com/emove/less/examples/chatroom/chat"
)

type session struct {
	id      uint64
	name    string
	channel less.Channel
}

type hub struct {
	mu       sync.RWMutex
	nextID   uint64
	sessions map[less.Channel]*session
}

func newHub() *hub {
	return &hub{sessions: make(map[less.Channel]*session)}
}

func (h *hub) register(ch less.Channel) {
	h.mu.Lock()
	defer h.mu.Unlock()

	id := atomic.AddUint64(&h.nextID, 1)
	h.sessions[ch] = &session{id: id, channel: ch}
}

func (h *hub) unregister(ch less.Channel) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	sess, ok := h.sessions[ch]
	if !ok {
		return "", false
	}
	delete(h.sessions, ch)
	return sess.name, sess.name != ""
}

func (h *hub) setName(ch less.Channel, name string) string {
	h.mu.Lock()
	defer h.mu.Unlock()

	sess, ok := h.sessions[ch]
	if !ok {
		id := atomic.AddUint64(&h.nextID, 1)
		sess = &session{id: id, channel: ch}
		h.sessions[ch] = sess
	}
	sess.name = strings.TrimSpace(name)
	return sess.name
}

func (h *hub) nameOf(ch less.Channel) (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sess, ok := h.sessions[ch]
	if !ok || sess.name == "" {
		return "", false
	}
	return sess.name, true
}

func (h *hub) broadcast(msg *chat.Message) {
	for _, ch := range h.channels() {
		if err := ch.Write(msg); err != nil {
			ch.Close(fmt.Errorf("broadcast write failed: %w", err))
		}
	}
}

func (h *hub) channels() []less.Channel {
	h.mu.RLock()
	defer h.mu.RUnlock()

	channels := make([]less.Channel, 0, len(h.sessions))
	for ch := range h.sessions {
		channels = append(channels, ch)
	}
	return channels
}
