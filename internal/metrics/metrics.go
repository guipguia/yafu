// Package metrics owns the Prometheus metrics yafu exposes at /metrics.
// HTTP middleware in internal/server records request counters and latency;
// the cluster prober records probe outcomes; main.go wires the registry
// collector that scrapes per-cluster gauges on demand.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const namespace = "yafu"

// HTTPRequestsTotal counts every HTTP request the API server handles.
var HTTPRequestsTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "http_requests_total",
		Help:      "Total HTTP requests handled, labelled by method, path, and status code.",
	},
	[]string{"method", "path", "status"},
)

// HTTPRequestDuration is a histogram of HTTP request durations in seconds.
var HTTPRequestDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request handling latency in seconds.",
		Buckets:   prometheus.DefBuckets,
	},
	[]string{"method", "path"},
)

// ProbeTotal counts cluster probes by outcome (ok|error). The cluster
// prober increments this on every probe.
var ProbeTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "cluster_probe_total",
		Help:      "Total cluster probes, labelled by cluster name and result.",
	},
	[]string{"cluster", "result"},
)

// RecordProbe increments ProbeTotal with success → "ok" or "error".
func RecordProbe(cluster string, success bool) {
	result := "ok"
	if !success {
		result = "error"
	}
	ProbeTotal.WithLabelValues(cluster, result).Inc()
}

// RegistryEntry is the snapshot-friendly view of one cluster, decoupled
// from the cluster.Registry interface so this package stays cycle-free.
type RegistryEntry struct {
	Name      string
	Reachable bool
	Apps      int
	Ready     int
	Failing   int
	Suspended int
	Sources   int
}

// RegistryProvider is implemented by anything that can snapshot the
// cluster registry's current state for collection on /metrics scrape.
type RegistryProvider interface {
	Snapshot() []RegistryEntry
}

// MustRegisterRegistry registers a Prometheus collector that publishes
// per-cluster gauges (reachable, apps, failing, suspended, sources)
// and a global yafu_clusters_total. Reads are pull-based — we read the
// snapshot only when /metrics is scraped.
func MustRegisterRegistry(p RegistryProvider) {
	prometheus.MustRegister(newRegistryCollector(p))
}

type registryCollector struct {
	provider     RegistryProvider
	clustersDesc *prometheus.Desc
	reachable    *prometheus.Desc
	apps         *prometheus.Desc
	ready        *prometheus.Desc
	failing      *prometheus.Desc
	suspended    *prometheus.Desc
	sources      *prometheus.Desc
}

func newRegistryCollector(p RegistryProvider) *registryCollector {
	clusterLabels := []string{"cluster"}
	return &registryCollector{
		provider:     p,
		clustersDesc: prometheus.NewDesc("yafu_clusters_total", "Number of registered clusters.", nil, nil),
		reachable:    prometheus.NewDesc("yafu_cluster_reachable", "1 if reachable, 0 otherwise.", clusterLabels, nil),
		apps:         prometheus.NewDesc("yafu_cluster_apps", "Total Flux apps on the cluster.", clusterLabels, nil),
		ready:        prometheus.NewDesc("yafu_cluster_apps_ready", "Apps with Ready=True.", clusterLabels, nil),
		failing:      prometheus.NewDesc("yafu_cluster_apps_failing", "Apps with Ready=False.", clusterLabels, nil),
		suspended:    prometheus.NewDesc("yafu_cluster_apps_suspended", "Apps with spec.suspend=true.", clusterLabels, nil),
		sources:      prometheus.NewDesc("yafu_cluster_sources", "Source resources (Git/Helm/OCI/Bucket) on the cluster.", clusterLabels, nil),
	}
}

func (c *registryCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.clustersDesc
	ch <- c.reachable
	ch <- c.apps
	ch <- c.ready
	ch <- c.failing
	ch <- c.suspended
	ch <- c.sources
}

func (c *registryCollector) Collect(ch chan<- prometheus.Metric) {
	entries := c.provider.Snapshot()
	ch <- prometheus.MustNewConstMetric(c.clustersDesc, prometheus.GaugeValue, float64(len(entries)))
	for _, e := range entries {
		v := 0.0
		if e.Reachable {
			v = 1.0
		}
		ch <- prometheus.MustNewConstMetric(c.reachable, prometheus.GaugeValue, v, e.Name)
		ch <- prometheus.MustNewConstMetric(c.apps, prometheus.GaugeValue, float64(e.Apps), e.Name)
		ch <- prometheus.MustNewConstMetric(c.ready, prometheus.GaugeValue, float64(e.Ready), e.Name)
		ch <- prometheus.MustNewConstMetric(c.failing, prometheus.GaugeValue, float64(e.Failing), e.Name)
		ch <- prometheus.MustNewConstMetric(c.suspended, prometheus.GaugeValue, float64(e.Suspended), e.Name)
		ch <- prometheus.MustNewConstMetric(c.sources, prometheus.GaugeValue, float64(e.Sources), e.Name)
	}
}
