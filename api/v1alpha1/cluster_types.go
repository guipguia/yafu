package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Environment classifies a cluster's role.
// +kubebuilder:validation:Enum=prod;staging;dev;edge;mgmt
type Environment string

const (
	EnvProd    Environment = "prod"
	EnvStaging Environment = "staging"
	EnvDev     Environment = "dev"
	EnvEdge    Environment = "edge"
	EnvMgmt    Environment = "mgmt"
)

// Condition types reported on Cluster status.
const (
	ConditionReady         = "Ready"
	ConditionReachable     = "Reachable"
	ConditionFluxInstalled = "FluxInstalled"
)

// KubeconfigSecretRef points at a Secret holding a kubeconfig.
type KubeconfigSecretRef struct {
	// Name of the Secret.
	Name string `json:"name"`

	// Namespace of the Secret. Defaults to the namespace yafu runs in.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Key inside the Secret that holds the kubeconfig bytes.
	// +kubebuilder:default=kubeconfig
	// +optional
	Key string `json:"key,omitempty"`
}

// ClusterConnection describes how to reach a cluster.
// Exactly one of KubeconfigSecretRef or InCluster must be set.
type ClusterConnection struct {
	// KubeconfigSecretRef references a Secret containing kubeconfig bytes.
	// +optional
	KubeconfigSecretRef *KubeconfigSecretRef `json:"kubeconfigSecretRef,omitempty"`

	// InCluster uses yafu's own ServiceAccount credentials. Useful when this
	// Cluster represents the home cluster yafu itself runs in.
	// +optional
	InCluster bool `json:"inCluster,omitempty"`
}

// ClusterSpec is the desired state of a Cluster.
type ClusterSpec struct {
	// DisplayName is the human-readable name shown in the UI. Defaults to metadata.name.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// Region is a free-form descriptor like "AWS · eu-west-1" or "on-prem · dc1".
	// +optional
	Region string `json:"region,omitempty"`

	// Environment groups clusters by role.
	// +optional
	Environment Environment `json:"environment,omitempty"`

	// FluxNamespace is where the Flux controllers run on the target cluster.
	// +kubebuilder:default=flux-system
	// +optional
	FluxNamespace string `json:"fluxNamespace,omitempty"`

	// Connection describes how yafu reaches this cluster.
	Connection ClusterConnection `json:"connection"`
}

// ClusterSummary is a snapshot of Flux resource counts on the target cluster,
// refreshed periodically by the controller.
type ClusterSummary struct {
	// Apps is the total of Kustomizations and HelmReleases.
	Apps int32 `json:"apps"`
	// Ready is the count whose Ready condition is True.
	Ready int32 `json:"ready"`
	// Failing is the count whose Ready condition is False (excluding suspended).
	Failing int32 `json:"failing"`
	// Suspended is the count with spec.suspend=true.
	Suspended int32 `json:"suspended"`
	// Sources is the total of GitRepository, HelmRepository, OCIRepository, Bucket.
	Sources int32 `json:"sources"`
}

// ClusterStatus is the observed state of a Cluster.
type ClusterStatus struct {
	// Conditions: Ready, Reachable, FluxInstalled.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// ObservedGeneration is the spec generation last reconciled.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// KubernetesVersion of the target cluster, when reachable.
	// +optional
	KubernetesVersion string `json:"kubernetesVersion,omitempty"`

	// FluxVersion detected on the target cluster, when installed.
	// +optional
	FluxVersion string `json:"fluxVersion,omitempty"`

	// Summary of Flux resource counts on the target cluster.
	// +optional
	Summary *ClusterSummary `json:"summary,omitempty"`

	// LastProbeTime is when the controller last refreshed the connection state.
	// +optional
	LastProbeTime *metav1.Time `json:"lastProbeTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=yclu
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Env",type=string,JSONPath=`.spec.environment`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Apps",type=integer,JSONPath=`.status.summary.apps`
// +kubebuilder:printcolumn:name="Failing",type=integer,JSONPath=`.status.summary.failing`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Cluster represents an external Kubernetes cluster managed by yafu.
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec,omitempty"`
	Status ClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterList is a list of Cluster resources.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}
