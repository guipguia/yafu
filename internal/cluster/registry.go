package cluster

import (
	"sync"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
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

	Client    client.WithWatch
	Discovery discovery.DiscoveryInterface
	Kube      kubernetes.Interface

	// Generation is the spec.generation of the Cluster CR this entry
	// was built from. CRDRegistry uses it to decide whether a fresh
	// reconcile needs to rebuild Clients (expensive — invalidates
	// watch streams and discovery cache) or can reuse the existing
	// entry. Zero in file-mode where the concept doesn't apply.
	Generation int64
	// BuiltAt is when Clients was constructed; combined with Generation
	// to force a periodic rebuild even when the spec hasn't changed,
	// so credential rotation in a kubeconfig Secret eventually takes
	// effect without waiting for an operator to bump the spec.
	BuiltAt time.Time

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
