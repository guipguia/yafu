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

// AppHistoryEntry is one entry in an application's revision history.
// HelmRelease entries come from status.history (one per Helm release
// version); Kustomization entries are derived from status.lastApplied
// (only the current revision is preserved on the resource itself).
type AppHistoryEntry struct {
	Revision   string `json:"revision"`
	Status     string `json:"status,omitempty"`
	Action     string `json:"action,omitempty"`
	AppVersion string `json:"appVersion,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	Current    bool   `json:"current"`
}

// AppHistoryResponse is the top-level shape of
// GET /api/v1/applications/{...}/history.
type AppHistoryResponse struct {
	AppID   string            `json:"appId"`
	Entries []AppHistoryEntry `json:"entries"`
	// Note carries an explanation when full history isn't available
	// (e.g. Kustomization only persists its current revision).
	Note string `json:"note,omitempty"`
}

// TreeNode is one resource from the application's inventory. v0.1 renders
// these flat; ownerRef-driven nesting (Deployment → ReplicaSet → Pod) is
// v0.3 work.
type TreeNode struct {
	Group   string `json:"group,omitempty"`
	Version string `json:"version,omitempty"`
	Kind    string `json:"kind"`
	Ns      string `json:"ns,omitempty"`
	Name    string `json:"name"`
	// ready | failing | progressing | notfound | unknown
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// TreeResponse is the top-level shape of GET /api/v1/applications/{...}/tree.
type TreeResponse struct {
	AppID string     `json:"appId"`
	Nodes []TreeNode `json:"nodes"`
	Note  string     `json:"note,omitempty"`
}

// ManifestResponse is the top-level shape of
// GET /api/v1/applications/{...}/manifest. YAML is the live object on the
// cluster with server-side noise (managedFields, uid, resourceVersion)
// stripped so it's diff-friendly.
type ManifestResponse struct {
	AppID string `json:"appId"`
	Kind  string `json:"kind"`
	YAML  string `json:"yaml"`
}

// ImageUpdate is one ImagePolicy joined with its referenced
// ImageRepository.
type ImageUpdate struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	ClusterID string `json:"clusterId"`
	Ns        string `json:"ns"`
	Image     string `json:"image"`
	LatestTag string `json:"latestTag,omitempty"`
	// Policy is one of "semver:RANGE", "alphabetical", "numerical", or "" if
	// the policy is malformed.
	Policy  string `json:"policy"`
	Status  string `json:"status"` // ready | failing | progressing
	Age     string `json:"age"`
	Message string `json:"message,omitempty"`
}

// ImageUpdatesResponse is the top-level shape of
// GET /api/v1/image-updates.
type ImageUpdatesResponse struct {
	Updates []ImageUpdate  `json:"updates"`
	Errors  []ClusterError `json:"errors,omitempty"`
}

// ManagedField is one entry from a resource's metadata.managedFields,
// flattened for the frontend.
type ManagedField struct {
	Manager   string `json:"manager"`
	Operation string `json:"operation"` // Apply | Update
	Time      string `json:"time,omitempty"`
	// Foreign is true when Manager is not one of the known Flux
	// controllers (kustomize-controller, helm-controller, …) — i.e.
	// likely a manual kubectl/k9s/edit and therefore drift.
	Foreign bool `json:"foreign"`
}

// DriftedResource is one Inventory entry with its current managers.
type DriftedResource struct {
	Group    string         `json:"group,omitempty"`
	Version  string         `json:"version,omitempty"`
	Kind     string         `json:"kind"`
	Ns       string         `json:"ns,omitempty"`
	Name     string         `json:"name"`
	// ready | drift | notfound | unknown
	Status   string         `json:"status"`
	Managers []ManagedField `json:"managers,omitempty"`
}

// DiffResponse is the top-level shape of GET /api/v1/applications/.../diff.
// Note explains the v0.1 limitation (field-ownership drift only; true
// Git-vs-cluster diff lands in v0.4 with kustomize-build / helm-render).
type DiffResponse struct {
	AppID     string            `json:"appId"`
	Resources []DriftedResource `json:"resources"`
	Note      string            `json:"note,omitempty"`
}

// PodInfo is one Pod that's a candidate target for log streaming. The
// Logs tab lets the user pick from this list.
type PodInfo struct {
	Ns         string   `json:"ns"`
	Name       string   `json:"name"`
	Phase      string   `json:"phase"` // Pending | Running | Succeeded | Failed | Unknown
	Containers []string `json:"containers"`
	Restarts   int32    `json:"restarts"`
	Age        string   `json:"age"`
}

// LogsResponse is the top-level shape of GET /api/v1/applications/.../logs.
// Pods is every Pod in the application's inventory namespaces; Logs is the
// raw last-N lines from the Selected pod (or the first available Pod when
// the caller didn't specify one). Truncated indicates the caller hit the
// per-request line cap.
type LogsResponse struct {
	AppID     string    `json:"appId"`
	Pods      []PodInfo `json:"pods"`
	Selected  string    `json:"selected,omitempty"` // ns/name of the pod whose logs were returned
	Container string    `json:"container,omitempty"`
	Logs      string    `json:"logs,omitempty"`
	Truncated bool      `json:"truncated,omitempty"`
	Note      string    `json:"note,omitempty"`
}
