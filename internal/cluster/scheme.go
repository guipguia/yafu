package cluster

import (
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imageautov1 "github.com/fluxcd/image-automation-controller/api/v1"
	imagereflv1 "github.com/fluxcd/image-reflector-controller/api/v1"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

// RemoteScheme returns a runtime.Scheme registered with every type yafu
// reads from a remote cluster: core/apps from client-go, plus the Flux
// source-, kustomize-, helm-, notification-, image-reflector- and
// image-automation-controller APIs.
func RemoteScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(sourcev1.AddToScheme(s))
	utilruntime.Must(kustomizev1.AddToScheme(s))
	utilruntime.Must(helmv2.AddToScheme(s))
	utilruntime.Must(notificationv1.AddToScheme(s))
	utilruntime.Must(notificationv1beta3.AddToScheme(s))
	utilruntime.Must(imagereflv1.AddToScheme(s))
	utilruntime.Must(imageautov1.AddToScheme(s))
	return s
}
