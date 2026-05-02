package cluster

import (
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// RemoteScheme returns a runtime.Scheme registered with every type yafu
// reads from a remote cluster: core/apps from client-go, plus the Flux
// source-, kustomize-, and helm-controller APIs.
func RemoteScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(sourcev1.AddToScheme(s))
	utilruntime.Must(kustomizev1.AddToScheme(s))
	utilruntime.Must(helmv2.AddToScheme(s))
	return s
}
