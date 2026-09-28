package main

import "sync"

// Hub fans out incoming messages to browsers following the live view (SSE).
type Hub struct {
	mu   sync.RWMutex
	subs map[chan *Entry]struct{}
}

func NewHub() *Hub {
	return &Hub{subs: make(map[chan *Entry]struct{})}
}

func (h *Hub) Subscribe() chan *Entry {
	ch := make(chan *Entry, 1024)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) Unsubscribe(ch chan *Entry) {
	h.mu.Lock()
	delete(h.subs, ch)
	h.mu.Unlock()
}

// Publish never blocks: a slow client simply misses messages.
func (h *Hub) Publish(e *Entry) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}
