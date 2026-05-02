package cluster

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileConfig_Explicit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "yafu.yaml")
	body := `clusters:
  - name: prod-eu-west
    displayName: prod-eu-west-1
    region: AWS · eu-west-1
    environment: prod
    fluxNamespace: flux-system
    context: prod-eu-west
  - name: dev
    environment: dev
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}
	if len(cfg.Clusters) != 2 {
		t.Fatalf("got %d clusters, want 2", len(cfg.Clusters))
	}
	if cfg.Clusters[0].Name != "prod-eu-west" || cfg.Clusters[0].DisplayName != "prod-eu-west-1" {
		t.Errorf("clusters[0] = %+v", cfg.Clusters[0])
	}
	if cfg.Clusters[0].Region != "AWS · eu-west-1" {
		t.Errorf("region = %q (Unicode round-trip)", cfg.Clusters[0].Region)
	}
	if cfg.Clusters[1].FluxNamespace != "" {
		t.Errorf("fluxNamespace should be empty (default applied at registry build time), got %q",
			cfg.Clusters[1].FluxNamespace)
	}
}

func TestLoadFileConfig_Discover(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "yafu.yaml")
	body := `discoverContexts:
  kubeconfig: /etc/yafu/kubeconfig
  defaults:
    fluxNamespace: flux
    environment: dev
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFileConfig(path)
	if err != nil {
		t.Fatalf("LoadFileConfig: %v", err)
	}
	if cfg.DiscoverContexts == nil {
		t.Fatal("discoverContexts should be set")
	}
	if cfg.DiscoverContexts.Kubeconfig != "/etc/yafu/kubeconfig" {
		t.Errorf("kubeconfig = %q", cfg.DiscoverContexts.Kubeconfig)
	}
	if cfg.DiscoverContexts.Defaults.FluxNamespace != "flux" {
		t.Errorf("defaults.fluxNamespace = %q", cfg.DiscoverContexts.Defaults.FluxNamespace)
	}
	if cfg.DiscoverContexts.Defaults.Environment != "dev" {
		t.Errorf("defaults.environment = %q", cfg.DiscoverContexts.Defaults.Environment)
	}
}

func TestLoadFileConfig_MissingFile(t *testing.T) {
	_, err := LoadFileConfig("/nonexistent/yafu.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDefaultIfEmpty(t *testing.T) {
	if got := defaultIfEmpty("", "fallback"); got != "fallback" {
		t.Errorf("got %q", got)
	}
	if got := defaultIfEmpty("explicit", "fallback"); got != "explicit" {
		t.Errorf("got %q", got)
	}
}
