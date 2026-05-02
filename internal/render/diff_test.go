package render

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// buildScheme returns a runtime scheme with the core+apps Kubernetes
// kinds registered — the fake client uses it to encode/decode
// objects.
func buildScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	return s
}

// asUnstructured wraps a typed object as an Unstructured with its
// GVK populated, which is what RenderKustomization / RenderHelmRelease
// hand back.
func asUnstructured(t *testing.T, obj runtime.Object, gvk schema.GroupVersionKind) unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	cv, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	u.Object = cv
	u.SetGroupVersionKind(gvk)
	return *u
}

func TestDiffResources_InSync(t *testing.T) {
	depl := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "podinfo", Namespace: "podinfo"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptrInt32(2)},
	}
	rendered := asUnstructured(t, depl, schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})

	// Live state matches the rendered state byte-for-byte.
	c := fake.NewClientBuilder().
		WithScheme(buildScheme()).
		WithObjects(depl).
		Build()

	res, err := DiffResources(context.Background(), c, DiffOptions{
		Rendered: []unstructured.Unstructured{rendered},
	})
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("got %d, want 1", len(res))
	}
	if res[0].Status != "in-sync" {
		t.Errorf("status = %q, want in-sync", res[0].Status)
	}
}

func TestDiffResources_Drifted(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptrInt32(2)},
	}
	rendered := asUnstructured(t, desired, gvk)

	live := desired.DeepCopy()
	live.Spec.Replicas = ptrInt32(5) // drift
	c := fake.NewClientBuilder().
		WithScheme(buildScheme()).
		WithObjects(live).
		Build()

	res, err := DiffResources(context.Background(), c, DiffOptions{
		Rendered: []unstructured.Unstructured{rendered},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Status != "drifted" {
		t.Errorf("status = %q, want drifted", res[0].Status)
	}
	if res[0].ReconcileWould != "update" {
		t.Errorf("reconcileWould = %q, want update", res[0].ReconcileWould)
	}
	if len(res[0].Hunks) != 1 || len(res[0].Hunks[0].Lines) == 0 {
		t.Errorf("expected hunks/lines populated, got %+v", res[0].Hunks)
	}
	// Sanity check: at least one del + one add.
	var sawAdd, sawDel bool
	for _, ln := range res[0].Hunks[0].Lines {
		if ln.Kind == "add" {
			sawAdd = true
		}
		if ln.Kind == "del" {
			sawDel = true
		}
	}
	if !sawAdd || !sawDel {
		t.Errorf("missing add/del lines: add=%v del=%v", sawAdd, sawDel)
	}
}

func TestDiffResources_MissingOnCluster(t *testing.T) {
	rendered := asUnstructured(t, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "ns"},
	}, schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"})

	c := fake.NewClientBuilder().WithScheme(buildScheme()).Build()
	res, err := DiffResources(context.Background(), c, DiffOptions{
		Rendered: []unstructured.Unstructured{rendered},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Status != "missing-on-cluster" {
		t.Errorf("status = %q, want missing-on-cluster", res[0].Status)
	}
	if res[0].ReconcileWould != "create" {
		t.Errorf("reconcileWould = %q, want create", res[0].ReconcileWould)
	}
}

func TestDiffResources_ExtraOnCluster(t *testing.T) {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	live := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: "ns"}}
	c := fake.NewClientBuilder().
		WithScheme(buildScheme()).
		WithObjects(live).
		Build()

	res, err := DiffResources(context.Background(), c, DiffOptions{
		Rendered: []unstructured.Unstructured{}, // nothing rendered
		Inventory: []ResourceKey{
			{GVK: gvk, Ns: "ns", Nm: "old"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != "extra-on-cluster" {
		t.Errorf("got %+v, want one extra-on-cluster", res)
	}
	if res[0].ReconcileWould != "delete" {
		t.Errorf("reconcileWould = %q, want delete", res[0].ReconcileWould)
	}
}

func TestDiffResources_ExtraGoneAlready(t *testing.T) {
	// Inventory says a CM "stale" used to exist, but it's already
	// pruned from the cluster — diff should not surface it.
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	c := fake.NewClientBuilder().WithScheme(buildScheme()).Build()

	res, err := DiffResources(context.Background(), c, DiffOptions{
		Rendered: []unstructured.Unstructured{},
		Inventory: []ResourceKey{
			{GVK: gvk, Ns: "ns", Nm: "stale"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 0 {
		t.Errorf("expected no results, got %+v", res)
	}
}

func TestDiffResources_StripsServerSideNoise(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}

	// Live carries managedFields + uid + resourceVersion + status —
	// the desired YAML has none of those. After the strip, both
	// sides should marshal to the same YAML and we get in-sync.
	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "ns"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptrInt32(2)},
	}
	rendered := asUnstructured(t, desired, gvk)

	live := desired.DeepCopy()
	live.UID = "abc-123"
	live.ResourceVersion = "999"
	live.Generation = 7
	// managedFields entries are validated strictly by the fake
	// client (apiVersion + fieldsType + fieldsV1 all required) —
	// we don't need it to validate the strip logic. UID +
	// resourceVersion + generation + status are sufficient.
	live.Status = appsv1.DeploymentStatus{ReadyReplicas: 2, Replicas: 2}

	c := fake.NewClientBuilder().
		WithScheme(buildScheme()).
		WithObjects(live).
		Build()

	res, err := DiffResources(context.Background(), c, DiffOptions{
		Rendered: []unstructured.Unstructured{rendered},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Status != "in-sync" {
		t.Errorf("status = %q, want in-sync (server-side noise should be stripped); hunks=%+v", res[0].Status, res[0].Hunks)
	}
}

func ptrInt32(v int32) *int32 { return &v }
