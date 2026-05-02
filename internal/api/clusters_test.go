package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestClusterHealthStatus(t *testing.T) {
	cases := []struct {
		name string
		in   cluster.Status
		want string
	}{
		{"unreachable", cluster.Status{Reachable: false}, "unreachable"},
		{"reachable but no flux", cluster.Status{Reachable: true, FluxInstalled: false}, "degraded"},
		{"failing apps", cluster.Status{Reachable: true, FluxInstalled: true, Summary: cluster.Summary{Failing: 2}}, "failing"},
		{"all healthy", cluster.Status{Reachable: true, FluxInstalled: true, Summary: cluster.Summary{Apps: 5, Ready: 5}}, "healthy"},
		{"only suspended", cluster.Status{Reachable: true, FluxInstalled: true, Summary: cluster.Summary{Apps: 3, Ready: 2, Suspended: 1}}, "healthy"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := clusterHealthStatus(c.in); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestClustersHandler_NilRegistry(t *testing.T) {
	h := &clustersHandler{registry: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	w := httptest.NewRecorder()

	h.list(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp types.ClustersResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Clusters) != 0 {
		t.Errorf("expected empty clusters, got %d", len(resp.Clusters))
	}
}

func TestClustersHandler_MapsDTO(t *testing.T) {
	e := &cluster.Entry{
		Name:        "prod-eu-west",
		DisplayName: "prod-eu-west-1",
		Region:      "AWS · eu-west-1",
		Environment: "prod",
	}
	e.SetStatus(cluster.Status{
		Reachable:         true,
		FluxInstalled:     true,
		KubernetesVersion: "v1.30.0",
		FluxVersion:       "v2.3.0",
		Summary: cluster.Summary{
			Apps: 41, Ready: 38, Failing: 2, Suspended: 1, Sources: 11,
		},
	})

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &clustersHandler{registry: reg}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	var resp types.ClustersResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(resp.Clusters))
	}
	c := resp.Clusters[0]
	if c.ID != "prod-eu-west" {
		t.Errorf("ID = %q, want prod-eu-west", c.ID)
	}
	if c.Name != "prod-eu-west-1" {
		t.Errorf("Name = %q, want prod-eu-west-1", c.Name)
	}
	if c.Region != "AWS · eu-west-1" {
		t.Errorf("Region = %q", c.Region)
	}
	if c.Env != "prod" {
		t.Errorf("Env = %q", c.Env)
	}
	if c.Status != "failing" { // failing > 0 => failing
		t.Errorf("Status = %q, want failing", c.Status)
	}
	if c.Apps != 41 || c.Ready != 38 || c.Failing != 2 || c.Suspended != 1 || c.Sources != 11 {
		t.Errorf("counts wrong: %+v", c)
	}
	if !c.Reachable || !c.FluxInstalled {
		t.Errorf("expected reachable + flux installed; got %+v", c)
	}
}
