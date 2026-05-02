package cluster

import "github.com/guipguia/yafu/internal/metrics"

// MetricsSnapshot adapts a Registry to the metrics.RegistryProvider
// interface so the metrics package can collect per-cluster gauges
// without importing this package (avoiding an import cycle).
func MetricsSnapshot(r Registry) metrics.RegistryProvider {
	return &metricsAdapter{r: r}
}

type metricsAdapter struct{ r Registry }

func (a *metricsAdapter) Snapshot() []metrics.RegistryEntry {
	entries := a.r.List()
	out := make([]metrics.RegistryEntry, 0, len(entries))
	for _, e := range entries {
		st := e.Status()
		out = append(out, metrics.RegistryEntry{
			Name:      e.Name,
			Reachable: st.Reachable,
			Apps:      st.Summary.Apps,
			Ready:     st.Summary.Ready,
			Failing:   st.Summary.Failing,
			Suspended: st.Summary.Suspended,
			Sources:   st.Summary.Sources,
		})
	}
	return out
}
