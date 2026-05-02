package controllers

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	yafuv1alpha1 "github.com/guipguia/yafu/api/v1alpha1"
	"github.com/guipguia/yafu/internal/cluster"
)

// TestReconcile_ReuseEntryWhenGenerationUnchanged is the regression
// pin for the watcher-churn fix: two consecutive Reconcile calls
// against the same Cluster spec must NOT swap the registered
// Entry's client.WithWatch out from under the watch.Manager.
// (Probe failures are expected — the fake config can't reach a
// real API server — but the entry must still be cached.)
func TestReconcile_ReuseEntryWhenGenerationUnchanged(t *testing.T) {
	r, reg, req := setupReconciler(t, 1)

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	first, ok := reg.Get("home")
	if !ok {
		t.Fatal("entry missing after first reconcile")
	}
	if first.Generation != 1 {
		t.Errorf("first generation = %d, want 1", first.Generation)
	}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	second, ok := reg.Get("home")
	if !ok {
		t.Fatal("entry missing after second reconcile")
	}

	if first.Client != second.Client {
		t.Error("Client pointer changed across reconciles — watchers would be torn down")
	}
	if !first.BuiltAt.Equal(second.BuiltAt) {
		t.Error("BuiltAt advanced across reconciles — cache was bypassed")
	}
}

// TestReconcile_RebuildOnGenerationBump asserts the cache is
// invalidated when the Cluster spec actually changes.
func TestReconcile_RebuildOnGenerationBump(t *testing.T) {
	r, reg, req := setupReconciler(t, 1)

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	first, _ := reg.Get("home")

	// Bump generation on the underlying CR.
	var cr yafuv1alpha1.Cluster
	_ = r.Get(context.Background(), types.NamespacedName{Name: "home"}, &cr)
	cr.Generation = 2
	if err := r.Update(context.Background(), &cr); err != nil {
		t.Fatalf("bump generation: %v", err)
	}

	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	second, _ := reg.Get("home")

	if first.Client == second.Client {
		t.Error("Client pointer unchanged after generation bump — rebuild skipped incorrectly")
	}
	if second.Generation != 2 {
		t.Errorf("second generation = %d, want 2", second.Generation)
	}
}

func setupReconciler(t *testing.T, gen int64) (*ClusterReconciler, *cluster.CRDRegistry, ctrl.Request) {
	t.Helper()
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(yafuv1alpha1.AddToScheme(scheme))

	cr := &yafuv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "home", Generation: gen},
		Spec: yafuv1alpha1.ClusterSpec{
			Connection: yafuv1alpha1.ClusterConnection{InCluster: true},
		},
	}
	homeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cr).
		WithStatusSubresource(&yafuv1alpha1.Cluster{}).
		Build()

	reg := cluster.NewCRDRegistry()
	r := &ClusterReconciler{
		Client:     homeClient,
		Registry:   reg,
		HomeConfig: &rest.Config{Host: "https://unreachable.invalid"},
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "home"}}
	return r, reg, req
}
