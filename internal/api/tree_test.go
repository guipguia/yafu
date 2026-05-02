package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/audit"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestParseInventoryID(t *testing.T) {
	cases := []struct {
		id      string
		version string
		want    inventoryRef
		ok      bool
	}{
		{"shop_checkout-api__Service", "v1", inventoryRef{ns: "shop", name: "checkout-api", group: "", kind: "Service", version: "v1"}, true},
		{"shop_checkout-api_apps_Deployment", "v1", inventoryRef{ns: "shop", name: "checkout-api", group: "apps", kind: "Deployment", version: "v1"}, true},
		{"_my-clusterrole_rbac.authorization.k8s.io_ClusterRole", "v1", inventoryRef{ns: "", name: "my-clusterrole", group: "rbac.authorization.k8s.io", kind: "ClusterRole", version: "v1"}, true},
		{"too_few", "v1", inventoryRef{}, false},
	}
	for _, c := range cases {
		got, ok := parseInventoryID(c.id, c.version)
		if ok != c.ok {
			t.Errorf("parseInventoryID(%q) ok=%v want %v", c.id, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseInventoryID(%q) = %+v, want %+v", c.id, got, c.want)
		}
	}
}

func TestUnstructuredStatus_ReadyCondition(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"conditions": []any{
				map[string]any{"type": "Ready", "status": string(metav1.ConditionTrue), "message": "ok"},
			},
		},
	}}
	if s, m := unstructuredStatus(obj); s != "ready" || m != "ok" {
		t.Errorf("got (%q, %q)", s, m)
	}

	obj.Object["status"].(map[string]any)["conditions"] = []any{
		map[string]any{"type": "Ready", "status": string(metav1.ConditionFalse), "message": "boom"},
	}
	if s, _ := unstructuredStatus(obj); s != "failing" {
		t.Errorf("expected failing, got %q", s)
	}
}

func TestUnstructuredStatus_DeploymentReplicas(t *testing.T) {
	// Deployment-like: status.readyReplicas vs status.replicas.
	obj := &unstructured.Unstructured{Object: map[string]any{
		"status": map[string]any{
			"replicas":      int64(4),
			"readyReplicas": int64(2),
		},
	}}
	if s, m := unstructuredStatus(obj); s != "progressing" || m != "2/4 ready" {
		t.Errorf("got (%q, %q)", s, m)
	}

	obj.Object["status"].(map[string]any)["readyReplicas"] = int64(4)
	if s, _ := unstructuredStatus(obj); s != "ready" {
		t.Errorf("expected ready when all replicas up, got %q", s)
	}
}

func TestUnstructuredStatus_FallbackReady(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{}}
	if s, _ := unstructuredStatus(obj); s != "ready" {
		t.Errorf("expected ready fallback, got %q", s)
	}
}

func TestTree_KustomizationFlatList(t *testing.T) {
	now := metav1.Now()
	k := mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now)
	k.Status.Inventory = &kustomizev1.ResourceInventory{
		Entries: []kustomizev1.ResourceRef{
			{ID: "shop_checkout-api__Service", Version: "v1"},
			{ID: "shop_checkout-api_apps_Deployment", Version: "v1"},
			{ID: "shop_missing__ConfigMap", Version: "v1"},
		},
	}
	// Seed the fake client with the Service + Deployment but NOT the ConfigMap
	// (so the tree shows one notfound entry).
	svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "checkout-api", Namespace: "shop"}}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout-api", Namespace: "shop"},
		Status:     appsv1.DeploymentStatus{Replicas: 4, ReadyReplicas: 4},
	}

	e := newTreeEntry("alpha", "Alpha", k, svc, dep)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "shop", "kind": "Kustomization", "name": "checkout-api",
	})
	w := httptest.NewRecorder()
	h.tree(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp types.TreeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Nodes) != 3 {
		t.Fatalf("got %d nodes, want 3", len(resp.Nodes))
	}

	by := map[string]types.TreeNode{}
	for _, n := range resp.Nodes {
		by[n.Kind+"/"+n.Name] = n
	}
	if got := by["Service/checkout-api"]; got.Status != "ready" {
		t.Errorf("Service status = %q, want ready", got.Status)
	}
	if got := by["Deployment/checkout-api"]; got.Status != "ready" {
		t.Errorf("Deployment status = %q (4/4 ready)", got.Status)
	}
	if got := by["ConfigMap/missing"]; got.Status != "notfound" {
		t.Errorf("missing ConfigMap status = %q, want notfound", got.Status)
	}
}

func TestTree_HelmReleaseHasNote(t *testing.T) {
	now := metav1.Now()
	hr := mkHelmRelease("monitoring", "observability", false, true, "58.3.0", now)

	e := newTestEntry("alpha", "Alpha", hr)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &applicationsHandler{registry: reg, policy: auth.DefaultAllowAllPolicy, audit: audit.Discard()}

	req := newMutationRequest(http.MethodGet, "/x", map[string]string{
		"cluster": "alpha", "ns": "observability", "kind": "HelmRelease", "name": "monitoring",
	})
	w := httptest.NewRecorder()
	h.tree(w, req)

	var resp types.TreeResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Nodes) != 0 {
		t.Errorf("expected empty nodes for HelmRelease in v0.1, got %d", len(resp.Nodes))
	}
	if resp.Note == "" {
		t.Error("expected v0.3 note for HelmRelease tree")
	}
}

// newTreeEntry seeds a fake controller-runtime client with the given
// objects (Kustomization + the resources its Inventory references).
func newTreeEntry(id, displayName string, objs ...client.Object) *cluster.Entry {
	cb := fake.NewClientBuilder().
		WithScheme(cluster.RemoteScheme()).
		WithStatusSubresource(&kustomizev1.Kustomization{}).
		WithObjects(objs...)
	e := &cluster.Entry{
		Name:          id,
		DisplayName:   displayName,
		FluxNamespace: "flux-system",
		Client:        cb.Build(),
		Discovery:     &fakeDiscovery{},
	}
	e.SetStatus(cluster.Status{Reachable: true, FluxInstalled: true})
	return e
}

// silence unused-imports when compiled standalone
var _ = appsv1.Deployment{}
var _ = corev1.Service{}
