package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestEventsHandler_FiltersFluxEvents(t *testing.T) {
	now := metav1.Now()
	objs := []client.Object{
		mkEvent("ev-1", "shop", "kustomize.toolkit.fluxcd.io/v1", "Kustomization", "checkout", corev1.EventTypeNormal, "ReconciliationSucceeded", "applied", now),
		mkEvent("ev-2", "shop", "helm.toolkit.fluxcd.io/v2", "HelmRelease", "payments", corev1.EventTypeWarning, "Failed", "boom", now),
		mkEvent("ev-3", "default", "v1", "Pod", "nginx", corev1.EventTypeNormal, "Started", "started", now),
		mkEvent("ev-4", "default", "apps/v1", "Deployment", "nginx", corev1.EventTypeWarning, "Unhealthy", "no", now),
	}
	e := newEventsEntry("alpha", "Alpha", objs...)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &eventsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp types.EventsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Events) != 2 {
		t.Fatalf("got %d events, want 2 (Flux only)", len(resp.Events))
	}
	for _, ev := range resp.Events {
		switch ev.Reason {
		case "ReconciliationSucceeded":
			if ev.Kind != "ok" {
				t.Errorf("Normal event should map to kind=ok, got %q", ev.Kind)
			}
		case "Failed":
			if ev.Kind != "err" {
				t.Errorf("Warning event should map to kind=err, got %q", ev.Kind)
			}
		default:
			t.Errorf("unexpected reason %q (non-Flux event leaked through filter)", ev.Reason)
		}
	}
}

func TestIsFluxEvent(t *testing.T) {
	cases := []struct {
		apiVersion string
		want       bool
	}{
		{"kustomize.toolkit.fluxcd.io/v1", true},
		{"helm.toolkit.fluxcd.io/v2", true},
		{"source.toolkit.fluxcd.io/v1", true},
		{"notification.toolkit.fluxcd.io/v1beta3", true},
		{"v1", false},
		{"apps/v1", false},
		{"", false},
	}
	for _, c := range cases {
		got := isFluxEvent(&corev1.Event{InvolvedObject: corev1.ObjectReference{APIVersion: c.apiVersion}})
		if got != c.want {
			t.Errorf("isFluxEvent(%q) = %v, want %v", c.apiVersion, got, c.want)
		}
	}
}

// ---------- fixtures ----------

func newEventsEntry(id, displayName string, objs ...client.Object) *cluster.Entry {
	builder := fake.NewClientBuilder().WithScheme(cluster.RemoteScheme())
	if len(objs) > 0 {
		builder = builder.WithObjects(objs...)
	}
	e := &cluster.Entry{
		Name:          id,
		DisplayName:   displayName,
		FluxNamespace: "flux-system",
		Client:        builder.Build(),
		Discovery:     &fakeDiscovery{},
	}
	e.SetStatus(cluster.Status{Reachable: true, FluxInstalled: true})
	return e
}

func mkEvent(name, ns, apiVersion, kind, objName, evType, reason, msg string, now metav1.Time) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, CreationTimestamp: now},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       objName,
			Namespace:  ns,
		},
		Type:          evType,
		Reason:        reason,
		Message:       msg,
		LastTimestamp: metav1.NewTime(now.Add(-1 * time.Second)),
		Source:        corev1.EventSource{Component: "kustomize-controller"},
	}
}
