package cluster

import (
	"sync"
	"time"

	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Status is a snapshot of a cluster's reachability and Flux deployment.
type Status struct {
	Reachable         bool
	FluxInstalled     bool
	KubernetesVersion string
	FluxVersion       string
	LastError         string
	LastProbe         time.Time
	Summary           Summary
}

// Summary mirrors v1alpha1.ClusterSummary in a registry-internal form.
type Summary struct {
	Apps      int
	Ready     int
	Failing   int
	Suspended int
	Sources   int
}

// Entry is a registered cluster the HTTP API can address.
type Entry struct {
	Name          string
	DisplayName   string
	Region        string
	Environment   string
	FluxNamespace string

	Client    client.Client
	Discovery discovery.DiscoveryInterface

	statusMu sync.RWMutex
	status   Status
}

// Status returns the latest probe snapshot.
func (e *Entry) Status() Status {
	e.statusMu.RLock()
	defer e.statusMu.RUnlock()
	return e.status
}

// SetStatus replaces the snapshot atomically.
func (e *Entry) SetStatus(s Status) {
	e.statusMu.Lock()
	defer e.statusMu.Unlock()
	e.status = s
}

// Registry resolves cluster names to Entry handles. Implementations are
// the file-backed registry (dev/CI) and the CRD-backed registry populated
// by the Cluster controller (production).
type Registry interface {
	List() []*Entry
	Get(name string) (*Entry, bool)
}
