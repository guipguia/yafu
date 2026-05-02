package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/yaml"
)

const defaultFluxNamespace = "flux-system"

// FileConfig is the schema of the YAML file passed via --config-file.
type FileConfig struct {
	// Clusters is an explicit list of registered clusters.
	Clusters []ClusterConfig `json:"clusters,omitempty"`

	// DiscoverContexts, when set, registers every context in a kubeconfig
	// as a cluster (named after the context) using the given defaults.
	DiscoverContexts *DiscoverConfig `json:"discoverContexts,omitempty"`
}

// ClusterConfig describes one entry under .clusters.
type ClusterConfig struct {
	Name          string `json:"name"`
	DisplayName   string `json:"displayName,omitempty"`
	Region        string `json:"region,omitempty"`
	Environment   string `json:"environment,omitempty"`
	FluxNamespace string `json:"fluxNamespace,omitempty"`
	Kubeconfig    string `json:"kubeconfig,omitempty"` // path; empty = default loading rules
	Context       string `json:"context,omitempty"`    // context within the kubeconfig
}

// DiscoverConfig describes auto-discovery from a kubeconfig.
type DiscoverConfig struct {
	Kubeconfig string           `json:"kubeconfig,omitempty"` // path; empty = default loading rules
	Defaults   ClusterDefaults  `json:"defaults,omitempty"`
}

// ClusterDefaults are applied to every auto-discovered cluster.
type ClusterDefaults struct {
	Environment   string `json:"environment,omitempty"`
	FluxNamespace string `json:"fluxNamespace,omitempty"`
}

// LoadFileConfig reads a YAML/JSON config file from disk.
func LoadFileConfig(path string) (*FileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}
	var cfg FileConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}
	return &cfg, nil
}

// FileRegistry resolves clusters loaded from a FileConfig and refreshes
// their probe status on a fixed interval.
type FileRegistry struct {
	entries  map[string]*Entry
	order    []string
	interval time.Duration
	logger   *slog.Logger
}

// NewFileRegistry builds clients for each cluster in cfg. Probe state is
// empty until Start is called.
func NewFileRegistry(cfg *FileConfig, logger *slog.Logger) (*FileRegistry, error) {
	if logger == nil {
		logger = slog.Default()
	}
	fr := &FileRegistry{
		entries:  make(map[string]*Entry),
		interval: 30 * time.Second,
		logger:   logger,
	}

	if cfg.DiscoverContexts != nil {
		if err := fr.addFromDiscovery(*cfg.DiscoverContexts); err != nil {
			return nil, err
		}
	}

	for _, c := range cfg.Clusters {
		if err := fr.addExplicit(c); err != nil {
			return nil, fmt.Errorf("cluster %q: %w", c.Name, err)
		}
	}

	return fr, nil
}

func (r *FileRegistry) addExplicit(c ClusterConfig) error {
	if c.Name == "" {
		return fmt.Errorf("cluster name is required")
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if c.Kubeconfig != "" {
		rules.ExplicitPath = c.Kubeconfig
	}
	overrides := &clientcmd.ConfigOverrides{}
	if c.Context != "" {
		overrides.CurrentContext = c.Context
	}
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
	if err != nil {
		return fmt.Errorf("build rest config: %w", err)
	}
	return r.addFromRESTConfig(c.Name, c, cfg)
}

func (r *FileRegistry) addFromDiscovery(d DiscoverConfig) error {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if d.Kubeconfig != "" {
		rules.ExplicitPath = d.Kubeconfig
	}
	rawCfg, err := rules.Load()
	if err != nil {
		return fmt.Errorf("load kubeconfig: %w", err)
	}
	for ctxName := range rawCfg.Contexts {
		overrides := &clientcmd.ConfigOverrides{CurrentContext: ctxName}
		cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides).ClientConfig()
		if err != nil {
			r.logger.Warn("skip context", "context", ctxName, "err", err)
			continue
		}
		cc := ClusterConfig{
			Name:          ctxName,
			Environment:   d.Defaults.Environment,
			FluxNamespace: defaultIfEmpty(d.Defaults.FluxNamespace, defaultFluxNamespace),
		}
		if err := r.addFromRESTConfig(ctxName, cc, cfg); err != nil {
			r.logger.Warn("skip context", "context", ctxName, "err", err)
			continue
		}
	}
	return nil
}

func (r *FileRegistry) addFromRESTConfig(name string, c ClusterConfig, cfg *rest.Config) error {
	clients, err := NewClients(cfg)
	if err != nil {
		return err
	}
	e := &Entry{
		Name:          name,
		DisplayName:   defaultIfEmpty(c.DisplayName, name),
		Region:        c.Region,
		Environment:   c.Environment,
		FluxNamespace: defaultIfEmpty(c.FluxNamespace, defaultFluxNamespace),
		Client:        clients.Client,
		Discovery:     clients.Discovery,
		Kube:          clients.Kube,
	}
	r.entries[name] = e
	r.order = append(r.order, name)
	return nil
}

// Start runs an initial probe and then refreshes on r.interval until ctx
// is cancelled.
func (r *FileRegistry) Start(ctx context.Context) {
	r.probeAll(ctx)
	t := time.NewTicker(r.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.probeAll(ctx)
		}
	}
}

func (r *FileRegistry) probeAll(ctx context.Context) {
	var wg sync.WaitGroup
	for _, name := range r.order {
		e := r.entries[name]
		wg.Add(1)
		go func() {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			s := Probe(cctx, e)
			e.SetStatus(s)
			if s.LastError != "" {
				r.logger.Warn("probe error", "cluster", e.Name, "err", s.LastError)
			}
		}()
	}
	wg.Wait()
}

// List returns entries in registration order.
func (r *FileRegistry) List() []*Entry {
	out := make([]*Entry, 0, len(r.order))
	for _, name := range r.order {
		out = append(out, r.entries[name])
	}
	return out
}

// Get returns the entry by name.
func (r *FileRegistry) Get(name string) (*Entry, bool) {
	e, ok := r.entries[name]
	return e, ok
}

func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
