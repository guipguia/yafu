package cluster

import "sync"

// CRDRegistry is the in-memory registry populated by the Cluster controller.
// The controller calls Set on every successful reconcile and Delete on
// Cluster CR removal.
type CRDRegistry struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	order   []string
}

// NewCRDRegistry returns an empty registry.
func NewCRDRegistry() *CRDRegistry {
	return &CRDRegistry{entries: make(map[string]*Entry)}
}

// List returns entries in insertion order.
func (r *CRDRegistry) List() []*Entry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Entry, 0, len(r.order))
	for _, name := range r.order {
		if e, ok := r.entries[name]; ok {
			out = append(out, e)
		}
	}
	return out
}

// Get returns the entry by name.
func (r *CRDRegistry) Get(name string) (*Entry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[name]
	return e, ok
}

// Set inserts or replaces the entry for name.
func (r *CRDRegistry) Set(name string, e *Entry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.entries[name]; !exists {
		r.order = append(r.order, name)
	}
	r.entries[name] = e
}

// Delete removes the entry for name.
func (r *CRDRegistry) Delete(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			return
		}
	}
}
