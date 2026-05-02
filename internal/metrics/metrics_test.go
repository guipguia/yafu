package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

type fakeProvider struct {
	entries []RegistryEntry
}

func (f *fakeProvider) Snapshot() []RegistryEntry { return f.entries }

func TestRegistryCollector_GathersGauges(t *testing.T) {
	p := &fakeProvider{
		entries: []RegistryEntry{
			{Name: "alpha", Reachable: true, Apps: 5, Ready: 4, Failing: 1, Suspended: 0, Sources: 2},
			{Name: "bravo", Reachable: false, Apps: 3, Ready: 0, Failing: 0, Suspended: 1, Sources: 0},
		},
	}
	// Use a fresh registry so the test doesn't depend on global state.
	reg := prometheus.NewRegistry()
	reg.MustRegister(newRegistryCollector(p))

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}

	got := map[string]map[string]float64{} // metric name -> cluster label -> value
	for _, mf := range families {
		for _, m := range mf.GetMetric() {
			cluster := ""
			for _, l := range m.GetLabel() {
				if l.GetName() == "cluster" {
					cluster = l.GetValue()
				}
			}
			v := 0.0
			switch {
			case m.GetGauge() != nil:
				v = m.GetGauge().GetValue()
			case m.GetCounter() != nil:
				v = m.GetCounter().GetValue()
			}
			if got[mf.GetName()] == nil {
				got[mf.GetName()] = map[string]float64{}
			}
			got[mf.GetName()][cluster] = v
		}
	}

	checks := []struct {
		metric  string
		cluster string
		want    float64
	}{
		{"yafu_clusters_total", "", 2},
		{"yafu_cluster_reachable", "alpha", 1},
		{"yafu_cluster_reachable", "bravo", 0},
		{"yafu_cluster_apps", "alpha", 5},
		{"yafu_cluster_apps_failing", "alpha", 1},
		{"yafu_cluster_apps_suspended", "bravo", 1},
		{"yafu_cluster_sources", "alpha", 2},
	}
	for _, c := range checks {
		if v, ok := got[c.metric][c.cluster]; !ok || v != c.want {
			t.Errorf("%s{cluster=%q} = %v (present=%v), want %v", c.metric, c.cluster, v, ok, c.want)
		}
	}
}

func TestRecordProbe_IncrementsCounter(t *testing.T) {
	beforeOK := counterValue(t, ProbeTotal.WithLabelValues("metricstest", "ok"))
	beforeErr := counterValue(t, ProbeTotal.WithLabelValues("metricstest", "error"))

	RecordProbe("metricstest", true)
	RecordProbe("metricstest", true)
	RecordProbe("metricstest", false)

	if got := counterValue(t, ProbeTotal.WithLabelValues("metricstest", "ok")); got-beforeOK != 2 {
		t.Errorf("ok delta = %v, want 2", got-beforeOK)
	}
	if got := counterValue(t, ProbeTotal.WithLabelValues("metricstest", "error")); got-beforeErr != 1 {
		t.Errorf("error delta = %v, want 1", got-beforeErr)
	}
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("counter write: %v", err)
	}
	return m.GetCounter().GetValue()
}
