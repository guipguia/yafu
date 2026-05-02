// Package v1alpha1 contains API Schema definitions for the yafu.io v1alpha1 API group.
// +groupName=yafu.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion identifies the yafu.io/v1alpha1 API group/version.
var GroupVersion = schema.GroupVersion{Group: "yafu.io", Version: "v1alpha1"}

// SchemeBuilder is used to register this group's types with a runtime.Scheme.
var SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

// AddToScheme adds the types in this group to the given scheme.
var AddToScheme = SchemeBuilder.AddToScheme

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(GroupVersion,
		&Cluster{},
		&ClusterList{},
	)
	metav1.AddToGroupVersion(s, GroupVersion)
	return nil
}
