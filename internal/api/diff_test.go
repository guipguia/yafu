package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/audit"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestClassifyManagers_NoEntries(t *testing.T) {
	out, status := classifyManagers(nil)
	if len(out) != 0 || status != "ready" {
		t.Errorf("got managers=%+v status=%q, want empty + ready", out, status)
	}
}

func TestClassifyManagers_FluxOnlyIsReady(t *testing.T) {
	now := metav1.Now()
	out, status := classifyManagers([]metav1.ManagedFieldsEntry{
		{Manager: "kustomize-controller", Operation: "Apply", Time: &now},
		{Manager: "helm-controller", Operation: "Apply", Time: &now},
	})
	if status != "ready" {
		t.Errorf("status = %q, want ready", status)
	}
	for _, m := range out {
		if m.Foreign {
			t.Errorf("manager %q flagged foreign incorrectly", m.Manager)
		}
	}
}

func TestClassifyManagers_ForeignIsDrift(t *testing.T) {
	now := metav1.Now()
	out, status := classifyManagers([]metav1.ManagedFieldsEntry{
		{Manager: "kustomize-controller", Operation: "Apply", Time: &now},
		{Manager: "kubectl-edit", Operation: "Update", Time: &now},
	})
	if status != "drift" {
		t.Errorf("status = %q, want drift", status)
	}
	gotForeign := false
	for _, m := range out {
		if m.Manager == "kubectl-edit" && m.Foreign && m.Operation == "Update" {
			gotForeign = true
		}
		if m.Time == "" && m.Manager != "" {
			t.Errorf("missing Time for manager %q", m.Manager)
		}
	}
	if !gotForeign {
		t.Errorf("kubectl-edit not flagged foreign in %+v", out)
	}
}

func TestClassifyManagers_StripsExeSuffix(t *testing.T) {
	out, _ := classifyManagers([]metav1.ManagedFieldsEntry{
		{Manager: "kustomize-controller.exe", Operation: "Apply"},
	})
	if out[0].Manager != "kustomize-controller" || out[0].Foreign {
		t.Errorf("expected exe-stripped + flux-recognised, got %+v", out[0])
	}
}

func TestFluxManagers_RecognisedSet(t *testing.T) {
	for _, m := range []string{
		"kustomize-controller",
		"helm-controller",
		"source-controller",
		"notification-controller",
		"image-reflector-controller",
		"image-automation-controller",
	} {
		if !fluxManagers[m] {
			t.Errorf("expected %q to be recognised as a Flux manager", m)
		}
	}
	for _, m := range []string{"kubectl-edit", "kubectl-apply", "k9s", "terraform"} {
		if fluxManagers[m] {
			t.Errorf("%q should NOT be a Flux manager", m)
		}
	}
}

// Handler-level happy path: empty inventory still returns the v0.1 note.
func TestDiff_EmptyInventoryReturnsNote(t *testing.T) {
	now := metav1.Now()
	k := mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now)
	// No inventory.

	e := newTreeEntry("alpha", "Alpha", k)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api",
	})
	w := httptest.NewRecorder()
	h.diff(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp types.DiffResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Resources) != 0 {
		t.Errorf("expected empty resources, got %d", len(resp.Resources))
	}
	if resp.Note == "" {
		t.Error("expected v0.1 note explaining the limitation")
	}
}

// silence unused import for kustomizev1 in case fixtures don't reference it.
var _ = kustomizev1.GroupVersion
