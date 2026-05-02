// Package types defines the JSON DTOs the HTTP API serves to the
// frontend. Keep these aligned with web/src/lib/types.ts — they are the
// API contract until OpenAPI codegen lands.
package types

// Cluster summarises a registered cluster for the Fleet view.
type Cluster struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Region        string `json:"region,omitempty"`
	Env           string `json:"env,omitempty"`
	Status        string `json:"status"` // healthy, degraded, failing, unreachable
	Apps          int    `json:"apps"`
	Ready         int    `json:"ready"`
	Failing       int    `json:"failing"`
	Suspended     int    `json:"suspended"`
	Sources       int    `json:"sources"`
	Version       string `json:"version,omitempty"`
	Reachable     bool   `json:"reachable"`
	FluxInstalled bool   `json:"fluxInstalled"`
	LastError     string `json:"lastError,omitempty"`
	Spark         []int  `json:"spark,omitempty"`
}

// ClustersResponse is the top-level shape of GET /api/v1/clusters.
type ClustersResponse struct {
	Clusters []Cluster `json:"clusters"`
}

// Application is a unified row for Kustomization / HelmRelease.
type Application struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"` // Kustomization, HelmRelease
	Ns        string `json:"ns"`
	Cluster   string `json:"cluster"`   // display name
	ClusterID string `json:"clusterId"` // registry id (URL-safe)
	Status    string `json:"status"`    // healthy, degraded, failing, progressing, paused
	Sync      string `json:"sync"`      // Synced, OutOfSync, Progressing, Suspended
	Source    string `json:"source"`
	Revision  string `json:"revision"`
	Age       string `json:"age"`
	Message   string `json:"message,omitempty"`
	Suspended bool   `json:"suspended"`
	Replicas  string `json:"replicas,omitempty"`
}

// ApplicationsResponse is the top-level shape of GET /api/v1/applications.
// Errors carries per-cluster failures so partial fan-out is observable.
type ApplicationsResponse struct {
	Applications []Application  `json:"applications"`
	Errors       []ClusterError `json:"errors,omitempty"`
}

// ClusterError is a single per-cluster failure during fan-out.
type ClusterError struct {
	Cluster string `json:"cluster"`
	Error   string `json:"error"`
}

// Source is a Flux source resource (Git/Helm/OCI/Bucket).
type Source struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"` // GitRepository | HelmRepository | OCIRepository | Bucket
	Ns        string `json:"ns"`
	Cluster   string `json:"cluster"`
	ClusterID string `json:"clusterId"`
	URL       string `json:"url"`
	Ref       string `json:"ref,omitempty"`
	Revision  string `json:"revision,omitempty"`
	Status    string `json:"status"` // healthy | degraded | failing | progressing
	Interval  string `json:"interval,omitempty"`
	Age       string `json:"age"`
	Message   string `json:"message,omitempty"`
}

// SourcesResponse is the top-level shape of GET /api/v1/sources.
type SourcesResponse struct {
	Sources []Source       `json:"sources"`
	Errors  []ClusterError `json:"errors,omitempty"`
}

// Alert is a Flux notification.toolkit.fluxcd.io Alert joined with its
// referenced Provider.
type Alert struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	ClusterID string `json:"clusterId"`
	Ns        string `json:"ns"`
	Provider  string `json:"provider"` // resolved provider type (slack, pagerduty, …) or "missing"
	Severity  string `json:"severity"` // info | error
	Target    string `json:"target,omitempty"`
	Status    string `json:"status"` // healthy | paused
	Suspended bool   `json:"suspended"`
	Age       string `json:"age"`
}

// AlertsResponse is the top-level shape of GET /api/v1/alerts.
type AlertsResponse struct {
	Alerts []Alert        `json:"alerts"`
	Errors []ClusterError `json:"errors,omitempty"`
}

// Event is a single k8s Event from one of the Flux controllers, mapped
// for the Activity timeline.
type Event struct {
	ID        string `json:"id"`
	T         string `json:"t"` // last-seen timestamp, formatted
	Cluster   string `json:"cluster"`
	ClusterID string `json:"clusterId"`
	Ns        string `json:"ns"`
	Kind      string `json:"kind"` // ok | warn | err
	Type      string `json:"type"` // Normal | Warning
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Object    string `json:"object"` // <Kind>/<name>
	Source    string `json:"source"` // event source component
}

// EventsResponse is the top-level shape of GET /api/v1/events.
type EventsResponse struct {
	Events []Event        `json:"events"`
	Errors []ClusterError `json:"errors,omitempty"`
}
