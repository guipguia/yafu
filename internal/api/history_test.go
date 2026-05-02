package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/audit"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestHistory_HelmReleaseRendersSnapshots(t *testing.T) {
	now := metav1.Now()
	hr := mkHelmRelease("monitoring", "observability", false, true, "58.3.0", now)
	hr.Status.History = helmv2.Snapshots{
		// Newest first by Flux convention.
		{Version: 3, ChartVersion: "58.3.0", AppVersion: "v3.0", Status: "deployed", Action: helmv2.ReleaseAction("upgrade"), LastDeployed: now},
		{Version: 2, ChartVersion: "58.2.0", AppVersion: "v2.9", Status: "superseded", Action: helmv2.ReleaseAction("upgrade"), LastDeployed: metav1.NewTime(now.Add(-1 * time.Hour))},
		{Version: 1, ChartVersion: "58.1.0", AppVersion: "v2.8", Status: "superseded", Action: helmv2.ReleaseAction("install"), LastDeployed: metav1.NewTime(now.Add(-1 * 24 * time.Hour))},
	}
	e := newTestEntry("alpha", "Alpha", hr)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "observability", "kind": "HelmRelease", "name": "monitoring",
	})
	w := httptest.NewRecorder()
	h.history(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp types.AppHistoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(resp.Entries))
	}
	if !resp.Entries[0].Current {
		t.Errorf("entry[0] should be current (latest deployed snapshot)")
	}
	if resp.Entries[0].Revision != "58.3.0 · v3" {
		t.Errorf("entry[0] revision = %q", resp.Entries[0].Revision)
	}
	if resp.Entries[1].Status != "superseded" {
		t.Errorf("entry[1] status = %q", resp.Entries[1].Status)
	}
	if resp.Entries[2].Action != "install" {
		t.Errorf("entry[2] action = %q (want install)", resp.Entries[2].Action)
	}
}

func TestHistory_KustomizationCurrentRevisionOnly(t *testing.T) {
	now := metav1.Now()
	k := mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now)
	e := newTestEntry("alpha", "Alpha", k)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api",
	})
	w := httptest.NewRecorder()
	h.history(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp types.AppHistoryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Entries) != 1 || !resp.Entries[0].Current {
		t.Errorf("expected single current entry, got %+v", resp.Entries)
	}
	if resp.Entries[0].Revision != "7f3c1d9" {
		t.Errorf("revision = %q, want short SHA", resp.Entries[0].Revision)
	}
	if resp.Note == "" {
		t.Error("Kustomization response should include the explanatory note")
	}
}

func TestHistory_RBACDeny(t *testing.T) {
	e := newTestEntry("alpha", "Alpha")
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	policy := auth.Policy{DefaultAction: auth.ActionDeny}
	h := &applicationsHandler{registry: reg, policy: policy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout",
	})
	w := httptest.NewRecorder()
	h.history(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestHistory_UnsupportedKind(t *testing.T) {
	e := newTestEntry("alpha", "Alpha")
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Pod", "name": "wat",
	})
	w := httptest.NewRecorder()
	h.history(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for unsupported kind", w.Code)
	}
}

// silence unused import when fixtures don't reference all packages
var _ = kustomizev1.GroupVersion
