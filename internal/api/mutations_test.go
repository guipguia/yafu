package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/guipguia/yafu/internal/audit"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestMutate_ReconcileSetsAnnotation(t *testing.T) {
	now := metav1.Now()
	k := mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now)
	e := newTestEntry("alpha", "Alpha", k)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodPost, "/api/v1/applications/alpha/shop/Kustomization/checkout-api/reconcile",
		map[string]string{"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api"})
	w := httptest.NewRecorder()
	h.reconcile(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", w.Code, w.Body.String())
	}

	var got kustomizev1.Kustomization
	if err := e.Client.Get(context.Background(), types.NamespacedName{Namespace: "shop", Name: "checkout-api"}, &got); err != nil {
		t.Fatalf("re-get: %v", err)
	}
	v, ok := got.GetAnnotations()[reconcileAnnotation]
	if !ok {
		t.Fatalf("annotation %q not set; annotations=%+v", reconcileAnnotation, got.GetAnnotations())
	}
	if _, err := time.Parse(time.RFC3339Nano, v); err != nil {
		t.Errorf("annotation value %q is not RFC3339Nano: %v", v, err)
	}
}

func TestMutate_SuspendThenResume(t *testing.T) {
	now := metav1.Now()
	k := mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now)
	e := newTestEntry("alpha", "Alpha", k)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	// Suspend.
	w := httptest.NewRecorder()
	h.suspend(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api",
	}))
	if w.Code != http.StatusAccepted {
		t.Fatalf("suspend status = %d", w.Code)
	}
	var afterSuspend kustomizev1.Kustomization
	_ = e.Client.Get(context.Background(), types.NamespacedName{Namespace: "shop", Name: "checkout-api"}, &afterSuspend)
	if !afterSuspend.Spec.Suspend {
		t.Fatal("expected spec.suspend=true after suspend")
	}

	// Resume.
	w = httptest.NewRecorder()
	h.resume(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api",
	}))
	if w.Code != http.StatusAccepted {
		t.Fatalf("resume status = %d", w.Code)
	}
	var afterResume kustomizev1.Kustomization
	_ = e.Client.Get(context.Background(), types.NamespacedName{Namespace: "shop", Name: "checkout-api"}, &afterResume)
	if afterResume.Spec.Suspend {
		t.Fatal("expected spec.suspend=false after resume")
	}
}

func TestMutate_HelmReleaseSuspend(t *testing.T) {
	now := metav1.Now()
	hr := mkHelmRelease("monitoring", "observability", false, true, "58.3.0", now)
	e := newTestEntry("alpha", "Alpha", hr)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	w := httptest.NewRecorder()
	h.suspend(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "observability", "kind": "HelmRelease", "name": "monitoring",
	}))
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var got helmv2.HelmRelease
	_ = e.Client.Get(context.Background(), types.NamespacedName{Namespace: "observability", Name: "monitoring"}, &got)
	if !got.Spec.Suspend {
		t.Fatal("expected HelmRelease spec.suspend=true")
	}
}

func TestMutate_RBACForbidden(t *testing.T) {
	e := newTestEntry("alpha", "Alpha")
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	policy := auth.Policy{
		DefaultAction: auth.ActionDeny,
		Rules: []auth.Rule{
			{Subjects: []string{"*"}, Verbs: []string{"get"}, Clusters: []string{"*"}, Action: auth.ActionAllow},
		},
	}
	h := &applicationsHandler{registry: reg, policy: policy, audit: audit.Discard()}

	w := httptest.NewRecorder()
	h.reconcile(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout",
	}))
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 (policy doesn't allow reconcile)", w.Code)
	}
}

func TestMutate_UnknownCluster(t *testing.T) {
	e := newTestEntry("alpha", "Alpha")
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	w := httptest.NewRecorder()
	h.reconcile(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "nope", "ns": "shop", "kind": "Kustomization", "name": "checkout",
	}))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for unknown cluster", w.Code)
	}
}

func TestMutate_UnsupportedKind(t *testing.T) {
	e := newTestEntry("alpha", "Alpha")
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	w := httptest.NewRecorder()
	h.reconcile(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Pod", "name": "wat",
	}))
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 for unsupported kind", w.Code)
	}
}

// newMutationRequest builds an httptest request with the given path
// values pre-populated. Required because we don't go through the
// ServeMux in handler-level tests.
func newMutationRequest(method, url string, pathValues map[string]string) *http.Request {
	req := httptest.NewRequest(method, url, nil)
	for k, v := range pathValues {
		req.SetPathValue(k, v)
	}
	return req
}

func TestMutate_AuditRecordEmittedOnSuccess(t *testing.T) {
	now := metav1.Now()
	k := mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now)
	e := newTestEntry("alpha", "Alpha", k)

	var buf bytes.Buffer
	auditLog := audit.New(&buf)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: auditLog}

	req := newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api",
	})
	req = req.WithContext(auth.WithIdentity(req.Context(), auth.Identity{
		Subject: "uid-42", Email: "maria@acme.example", Groups: []string{"platform-admins"},
	}))
	w := httptest.NewRecorder()
	h.reconcile(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d", w.Code)
	}

	var rec audit.Record
	line := strings.TrimSpace(buf.String())
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("audit line not JSON: %v\n%s", err, line)
	}
	if rec.Verb != "reconcile" || rec.Outcome != audit.OutcomeOK {
		t.Errorf("rec = %+v, want verb=reconcile outcome=ok", rec)
	}
	if rec.Identity.Subject != "uid-42" || rec.Identity.Email != "maria@acme.example" {
		t.Errorf("identity wrong in audit: %+v", rec.Identity)
	}
	if rec.Resource.Cluster != "alpha" || rec.Resource.Name != "checkout-api" {
		t.Errorf("resource wrong in audit: %+v", rec.Resource)
	}
}

func TestMutate_AuditRecordEmittedOnDeny(t *testing.T) {
	e := newTestEntry("alpha", "Alpha")
	policy := auth.Policy{DefaultAction: auth.ActionDeny}
	var buf bytes.Buffer
	auditLog := audit.New(&buf)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: policy, audit: auditLog}

	w := httptest.NewRecorder()
	h.reconcile(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api",
	}))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}

	var rec audit.Record
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &rec); err != nil {
		t.Fatalf("audit line not JSON: %v", err)
	}
	if rec.Outcome != audit.OutcomeDenied {
		t.Errorf("outcome = %q, want denied", rec.Outcome)
	}
}
