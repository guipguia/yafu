package cluster

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewClients builds a controller-runtime client and a discovery client
// for the given REST config, registered with the remote scheme.
func NewClients(cfg *rest.Config) (client.Client, discovery.DiscoveryInterface, error) {
	c, err := client.New(cfg, client.Options{Scheme: RemoteScheme()})
	if err != nil {
		return nil, nil, fmt.Errorf("build controller-runtime client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("build discovery client: %w", err)
	}
	return c, disco, nil
}
