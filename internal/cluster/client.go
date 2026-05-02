package cluster

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Clients bundles the three handles yafu needs to talk to a remote cluster:
//
//	Client    — controller-runtime typed client for List/Get/Patch
//	Discovery — version + Flux installation probe
//	Kube      — clientset, used for subresources controller-runtime can't
//	            reach (today: Pod logs)
type Clients struct {
	Client    client.Client
	Discovery discovery.DiscoveryInterface
	Kube      kubernetes.Interface
}

// NewClients builds the three clients for the given REST config, registered
// with the remote scheme.
func NewClients(cfg *rest.Config) (Clients, error) {
	c, err := client.New(cfg, client.Options{Scheme: RemoteScheme()})
	if err != nil {
		return Clients{}, fmt.Errorf("build controller-runtime client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return Clients{}, fmt.Errorf("build discovery client: %w", err)
	}
	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return Clients{}, fmt.Errorf("build kubernetes clientset: %w", err)
	}
	return Clients{Client: c, Discovery: disco, Kube: kube}, nil
}
