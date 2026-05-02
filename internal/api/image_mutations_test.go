package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	imageautov1 "github.com/fluxcd/image-automation-controller/api/v1"
	imagereflv1 "github.com/fluxcd/image-reflector-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/guipguia/yafu/internal/audit"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func newImageEntry(id string, objs ...any) *cluster.Entry {
	builder := fake.NewClientBuilder().WithScheme(cluster.RemoteScheme())
	for _, o := range objs {
		switch v := o.(type) {
		case *imagereflv1.ImagePolicy:
			builder = builder.WithObjects(v)
		case *imagereflv1.ImageRepository:
			builder = builder.WithObjects(v)
		case *imageautov1.ImageUpdateAutomation:
			builder = builder.WithObjects(v)
		}
	}
	e := &cluster.Entry{
		Name:          id,
		DisplayName:   id,
		FluxNamespace: "flux-system",
		Client:        builder.Build(),
		Discovery:     &fakeDiscovery{info: nil},
	}
	e.SetStatus(cluster.Status{Reachable: true, FluxInstalled: true})
	return e
}

func mkSimpleImagePolicy(name, ns string, suspend bool) *imagereflv1.ImagePolicy {
	return &imagereflv1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec:       imagereflv1.ImagePolicySpec{Suspend: suspend},
	}
}

func TestImageMutate_ReconcileSetsAnnotation(t *testing.T) {
	p := mkSimpleImagePolicy("checkout", "apps", false)
	e := newImageEntry("alpha", p)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &imageMutationsHandler{deps: mutationDeps{
		registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard(),
	}}

	w := httptest.NewRecorder()
	h.reconcile(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "apps", "kind": "ImagePolicy", "name": "checkout",
	}))
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}

	var got imagereflv1.ImagePolicy
	if err := e.Client.Get(context.Background(), types.NamespacedName{Namespace: "apps", Name: "checkout"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	v, ok := got.GetAnnotations()[reconcileAnnotation]
	if !ok {
		t.Fatalf("annotation not set; annotations=%+v", got.GetAnnotations())
	}
	if _, err := time.Parse(time.RFC3339Nano, v); err != nil {
		t.Errorf("annotation %q is not RFC3339Nano: %v", v, err)
	}
}

func TestImageMutate_SuspendThenResume(t *testing.T) {
	p := mkSimpleImagePolicy("checkout", "apps", false)
	e := newImageEntry("alpha", p)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &imageMutationsHandler{deps: mutationDeps{
		registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard(),
	}}

	pv := map[string]string{"cluster": "alpha", "ns": "apps", "kind": "ImagePolicy", "name": "checkout"}

	w := httptest.NewRecorder()
	h.suspend(w, newMutationRequest(http.MethodPost, "/x", pv))
	if w.Code != http.StatusAccepted {
		t.Fatalf("suspend: %d", w.Code)
	}
	var got imagereflv1.ImagePolicy
	_ = e.Client.Get(context.Background(), types.NamespacedName{Namespace: "apps", Name: "checkout"}, &got)
	if !got.Spec.Suspend {
		t.Fatal("expected suspend=true")
	}

	w = httptest.NewRecorder()
	h.resume(w, newMutationRequest(http.MethodPost, "/x", pv))
	if w.Code != http.StatusAccepted {
		t.Fatalf("resume: %d", w.Code)
	}
	_ = e.Client.Get(context.Background(), types.NamespacedName{Namespace: "apps", Name: "checkout"}, &got)
	if got.Spec.Suspend {
		t.Fatal("expected suspend=false after resume")
	}
}

func TestImageMutate_ImageUpdateAutomation(t *testing.T) {
	a := &imageautov1.ImageUpdateAutomation{
		ObjectMeta: metav1.ObjectMeta{Name: "auto", Namespace: "flux-system"},
	}
	e := newImageEntry("alpha", a)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &imageMutationsHandler{deps: mutationDeps{
		registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard(),
	}}

	w := httptest.NewRecorder()
	h.suspend(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "flux-system", "kind": "ImageUpdateAutomation", "name": "auto",
	}))
	if w.Code != http.StatusAccepted {
		t.Fatalf("suspend: %d body=%s", w.Code, w.Body.String())
	}
	var got imageautov1.ImageUpdateAutomation
	_ = e.Client.Get(context.Background(), types.NamespacedName{Namespace: "flux-system", Name: "auto"}, &got)
	if !got.Spec.Suspend {
		t.Fatal("expected automation suspend=true")
	}
}

func TestImageMutate_UnknownKind(t *testing.T) {
	e := newImageEntry("alpha")
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &imageMutationsHandler{deps: mutationDeps{
		registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard(),
	}}

	w := httptest.NewRecorder()
	h.reconcile(w, newMutationRequest(http.MethodPost, "/x", map[string]string{
		"cluster": "alpha", "ns": "ns", "kind": "Pod", "name": "thing",
	}))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (unsupported kind); body=%s", w.Code, w.Body.String())
	}
}
