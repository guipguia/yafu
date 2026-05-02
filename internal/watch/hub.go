// Package watch wires Kubernetes watch streams across the registered
// clusters into a single in-process event hub. SSE consumers subscribe
// to the hub to learn when Flux resources change so the frontend can
// invalidate cached queries instead of polling.
package watch

import "sync"

// Event is a coarse change notification. We deliberately don't carry
// the full object payload — clients use it to invalidate a cache and
// re-fetch through the regular list endpoints (which already apply
// RBAC, fan-out, and serialisation).
type Event struct {
	Cluster string `json:"cluster"`
	Kind    string `json:"kind"`
	// Verb is the watch.Event.Type value: "ADDED", "MODIFIED", "DELETED",
	// or "BOOKMARK". Frontends generally only need to know "something
	// changed for this kind on this cluster", but we forward the verb
	// so consumers can be more selective if they want to.
	Verb string `json:"verb"`
	Ns   string `json:"namespace,omitempty"`
	Name string `json:"name"`
}

// Hub fans events from per-cluster watchers out to subscribed SSE
// clients. It is safe for concurrent use; subscribers that fall
// behind have events dropped rather than blocking publishers.
type Hub struct {
	mu   sync.RWMutex
	subs map[*subscriber]struct{}
}

type subscriber struct {
	ch chan Event
}

// NewHub returns an empty Hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[*subscriber]struct{})}
}

// Subscribe registers a new consumer and returns its channel and an
// unsubscribe function. The unsubscribe function is idempotent and
// closes the channel — callers should defer it.
func (h *Hub) Subscribe() (<-chan Event, func()) {
	s := &subscriber{ch: make(chan Event, 64)}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	unsub := func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs, s)
			h.mu.Unlock()
			close(s.ch)
		})
	}
	return s.ch, unsub
}

// Publish delivers ev to every subscriber. Slow subscribers (full
// buffer) drop the event rather than block — losing an invalidation
// just means the client refetches a few seconds late on its polling
// fallback, which is acceptable.
func (h *Hub) Publish(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		select {
		case s.ch <- ev:
		default:
		}
	}
}
