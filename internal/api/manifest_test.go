package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/audit"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestManifest_RendersYAML(t *testing.T) {
	now := metav1.Now()
	k := mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now)
	// Pre-populate noisy server-managed fields to verify they're stripped.
	k.UID = "00000000-1111-2222-3333-444444444444"
	k.ResourceVersion = "12345"
	k.Generation = 7
	k.CreationTimestamp = now

	e := newTestEntry("alpha", "Alpha", k)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api",
	})
	w := httptest.NewRecorder()
	h.manifest(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp types.ManifestResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Kind != "Kustomization" {
		t.Errorf("kind = %q", resp.Kind)
	}

	// Body should be valid YAML containing apiVersion + kind + name.
	for _, want := range []string{
		"apiVersion: kustomize.toolkit.fluxcd.io/v1",
		"kind: Kustomization",
		"name: checkout-api",
		"namespace: shop",
	} {
		if !strings.Contains(resp.YAML, want) {
			t.Errorf("YAML missing %q\n----\n%s", want, resp.YAML)
		}
	}

	// Server-managed noise must be gone.
	for _, banned := range []string{
		"managedFields",
		"uid:",
		"resourceVersion:",
		"generation:",
		"creationTimestamp:",
	} {
		if strings.Contains(resp.YAML, banned) {
			t.Errorf("YAML still contains noise field %q\n----\n%s", banned, resp.YAML)
		}
	}
}

func TestManifest_NotFound(t *testing.T) {
	e := newTestEntry("alpha", "Alpha")
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "nope",
	})
	w := httptest.NewRecorder()
	h.manifest(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestManifest_RBACDeny(t *testing.T) {
	e := newTestEntry("alpha", "Alpha")
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	policy := auth.Policy{DefaultAction: auth.ActionDeny}
	h := &applicationsHandler{registry: reg, policy: policy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "anything",
	})
	w := httptest.NewRecorder()
	h.manifest(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}
