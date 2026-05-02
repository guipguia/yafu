package cluster

import (
	"context"
	"errors"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeDiscovery struct {
	discovery.DiscoveryInterface
	info *version.Info
	err  error
}

func (f *fakeDiscovery) ServerVersion() (*version.Info, error) { return f.info, f.err }

func TestProbe_Unreachable(t *testing.T) {
	e := &Entry{
		Discovery: &fakeDiscovery{err: errors.New("connection refused")},
	}
	s := Probe(context.Background(), e)
	if s.Reachable {
		t.Error("expected !Reachable")
	}
	if s.LastError == "" {
		t.Error("expected LastError set")
	}
	if s.FluxInstalled {
		t.Error("expected !FluxInstalled when unreachable")
	}
}

func TestProbe_FluxNotInstalled(t *testing.T) {
	// Build the fake client with ONLY the core scheme; listing Kustomizations
	// then returns a NoKindMatchError which Probe should treat as "Flux not
	// installed", not as a probe error.
	core := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(core); err != nil {
		t.Fatal(err)
	}
	e := &Entry{
		Client:        fake.NewClientBuilder().WithScheme(core).Build(),
		Discovery:     &fakeDiscovery{info: &version.Info{GitVersion: "v1.30.0"}},
		FluxNamespace: "flux-system",
	}
	s := Probe(context.Background(), e)
	if !s.Reachable {
		t.Error("expected Reachable")
	}
	if s.FluxInstalled {
		t.Error("expected !FluxInstalled (CRDs absent)")
	}
	if s.KubernetesVersion != "v1.30.0" {
		t.Errorf("KubernetesVersion = %q, want v1.30.0", s.KubernetesVersion)
	}
	if s.LastError != "" {
		t.Errorf("LastError = %q, want empty (NoKindMatch is not a probe error)", s.LastError)
	}
}

func TestProbe_Counts(t *testing.T) {
	now := metav1.Now()
	objs := []runtime.Object{
		mkKust(t, "ready", false, true, now),
		mkKust(t, "failing", false, false, now),
		mkKust(t, "suspended", true, true, now),
		mkHR(t, "hr-ready", false, true, now),
		&sourcev1.GitRepository{ObjectMeta: metav1.ObjectMeta{Name: "g1", Namespace: "flux-system"}},
		&sourcev1.HelmRepository{ObjectMeta: metav1.ObjectMeta{Name: "h1", Namespace: "flux-system"}},
		&sourcev1.OCIRepository{ObjectMeta: metav1.ObjectMeta{Name: "o1", Namespace: "flux-system"}},
		&sourcev1.Bucket{ObjectMeta: metav1.ObjectMeta{Name: "b1", Namespace: "flux-system"}},
	}
	cl := fake.NewClientBuilder().
		WithScheme(RemoteScheme()).
		WithStatusSubresource(&kustomizev1.Kustomization{}, &helmv2.HelmRelease{}).
		WithRuntimeObjects(objs...).
		Build()

	e := &Entry{
		Client:        cl,
		Discovery:     &fakeDiscovery{info: &version.Info{GitVersion: "v1.30.0"}},
		FluxNamespace: "flux-system",
	}
	s := Probe(context.Background(), e)

	if !s.FluxInstalled {
		t.Fatal("expected FluxInstalled")
	}
	if s.Summary.Apps != 4 {
		t.Errorf("Apps = %d, want 4 (3 Kust + 1 HR)", s.Summary.Apps)
	}
	if s.Summary.Ready != 2 {
		t.Errorf("Ready = %d, want 2 (1 Kust + 1 HR)", s.Summary.Ready)
	}
	if s.Summary.Failing != 1 {
		t.Errorf("Failing = %d, want 1", s.Summary.Failing)
	}
	if s.Summary.Suspended != 1 {
		t.Errorf("Suspended = %d, want 1", s.Summary.Suspended)
	}
	if s.Summary.Sources != 4 {
		t.Errorf("Sources = %d, want 4 (git+helm+oci+bucket)", s.Summary.Sources)
	}
}

func TestIsReady(t *testing.T) {
	cases := []struct {
		name string
		in   []metav1.Condition
		want bool
	}{
		{"empty", nil, false},
		{"ready true", []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}, true},
		{"ready false", []metav1.Condition{{Type: "Ready", Status: metav1.ConditionFalse}}, false},
		{"other only", []metav1.Condition{{Type: "Reconciling", Status: metav1.ConditionTrue}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isReady(c.in); got != c.want {
				t.Errorf("isReady = %v, want %v", got, c.want)
			}
		})
	}
}

// ---------- fixtures ----------

func mkKust(t *testing.T, name string, suspended, ready bool, now metav1.Time) *kustomizev1.Kustomization {
	t.Helper()
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	return &kustomizev1.Kustomization{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "flux-system"},
		Spec:       kustomizev1.KustomizationSpec{Suspend: suspended},
		Status: kustomizev1.KustomizationStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "x"},
			},
		},
	}
}

func mkHR(t *testing.T, name string, suspended, ready bool, now metav1.Time) *helmv2.HelmRelease {
	t.Helper()
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	return &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "flux-system"},
		Spec:       helmv2.HelmReleaseSpec{Suspend: suspended},
		Status: helmv2.HelmReleaseStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "x"},
			},
		},
	}
}
