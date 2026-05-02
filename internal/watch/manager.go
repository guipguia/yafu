package watch

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guipguia/yafu/internal/cluster"
)

// Manager owns the lifecycle of one ClusterWatcher per registered
// cluster. It re-syncs against the registry on a tick: new clusters
// get watchers started; removed clusters have their watchers
// cancelled. A new client.WithWatch on an existing entry (e.g.
// because the CRD reconciler rebuilt clients) replaces the running
// watcher so we don't keep a stream against a stale connection.
type Manager struct {
	Hub      *Hub
	Registry cluster.Registry
	Logger   *slog.Logger

	// Interval between resyncs. Zero defaults to 15s.
	Interval time.Duration

	mu      sync.Mutex
	running map[string]*runningWatcher
}

type runningWatcher struct {
	cancel context.CancelFunc
	// client is the WithWatch the active watcher was started with;
	// used as identity to detect when the registry has handed us a
	// new client for the same cluster name.
	client client.WithWatch
}

// Run blocks until ctx is cancelled, doing one initial sync and then
// re-syncing every Interval.
func (m *Manager) Run(ctx context.Context) {
	if m.running == nil {
		m.running = make(map[string]*runningWatcher)
	}
	if m.Logger == nil {
		m.Logger = slog.Default()
	}
	interval := m.Interval
	if interval == 0 {
		interval = 15 * time.Second
	}

	m.sync(ctx)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			m.shutdownAll()
			return
		case <-t.C:
			m.sync(ctx)
		}
	}
}

func (m *Manager) sync(parent context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seen := make(map[string]struct{}, len(m.running))
	for _, e := range m.Registry.List() {
		seen[e.Name] = struct{}{}
		existing, ok := m.running[e.Name]
		if ok && existing.client == e.Client {
			continue
		}
		// Either no watcher yet, or the client identity changed —
		// stop the old one and start fresh.
		if ok {
			existing.cancel()
			delete(m.running, e.Name)
		}
		ctx, cancel := context.WithCancel(parent)
		cw := &ClusterWatcher{
			Cluster: e.Name,
			Client:  e.Client,
			Hub:     m.Hub,
			Logger:  m.Logger.With("cluster", e.Name),
		}
		go cw.Run(ctx)
		m.running[e.Name] = &runningWatcher{cancel: cancel, client: e.Client}
	}
	for name, rw := range m.running {
		if _, ok := seen[name]; !ok {
			rw.cancel()
			delete(m.running, name)
		}
	}
}

func (m *Manager) shutdownAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, rw := range m.running {
		rw.cancel()
	}
	m.running = map[string]*runningWatcher{}
}
