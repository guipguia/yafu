// Hand-written deepcopy methods for the v1alpha1 types. Mirrors what
// controller-gen would produce; replace with generated output if the schema
// grows or controller-gen is added to the toolchain.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto copies the receiver into out.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy returns a deep-copy of the Cluster.
func (in *Cluster) DeepCopy() *Cluster {
	if in == nil {
		return nil
	}
	out := new(Cluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *Cluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out.
func (in *ClusterList) DeepCopyInto(out *ClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Cluster, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy returns a deep-copy of the ClusterList.
func (in *ClusterList) DeepCopy() *ClusterList {
	if in == nil {
		return nil
	}
	out := new(ClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *ClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies the receiver into out.
func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) {
	*out = *in
	in.Connection.DeepCopyInto(&out.Connection)
}

// DeepCopy returns a deep-copy of the ClusterSpec.
func (in *ClusterSpec) DeepCopy() *ClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *ClusterConnection) DeepCopyInto(out *ClusterConnection) {
	*out = *in
	if in.KubeconfigSecretRef != nil {
		out.KubeconfigSecretRef = new(KubeconfigSecretRef)
		*out.KubeconfigSecretRef = *in.KubeconfigSecretRef
	}
}

// DeepCopy returns a deep-copy of the ClusterConnection.
func (in *ClusterConnection) DeepCopy() *ClusterConnection {
	if in == nil {
		return nil
	}
	out := new(ClusterConnection)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *KubeconfigSecretRef) DeepCopyInto(out *KubeconfigSecretRef) {
	*out = *in
}

// DeepCopy returns a deep-copy of the KubeconfigSecretRef.
func (in *KubeconfigSecretRef) DeepCopy() *KubeconfigSecretRef {
	if in == nil {
		return nil
	}
	out := new(KubeconfigSecretRef)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *ClusterStatus) DeepCopyInto(out *ClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&out.Conditions[i])
		}
	}
	if in.Summary != nil {
		out.Summary = new(ClusterSummary)
		*out.Summary = *in.Summary
	}
	if in.LastProbeTime != nil {
		out.LastProbeTime = in.LastProbeTime.DeepCopy()
	}
}

// DeepCopy returns a deep-copy of the ClusterStatus.
func (in *ClusterStatus) DeepCopy() *ClusterStatus {
	if in == nil {
		return nil
	}
	out := new(ClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto copies the receiver into out.
func (in *ClusterSummary) DeepCopyInto(out *ClusterSummary) {
	*out = *in
}

// DeepCopy returns a deep-copy of the ClusterSummary.
func (in *ClusterSummary) DeepCopy() *ClusterSummary {
	if in == nil {
		return nil
	}
	out := new(ClusterSummary)
	in.DeepCopyInto(out)
	return out
}
