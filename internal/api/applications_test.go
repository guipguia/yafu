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
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

// ---------- pure helpers ----------

func TestShortRevision(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"main@7f3c1d9", "7f3c1d9"},
		{"main@7f3c1d9abcdef0123456789", "7f3c1d9"},
		{"v1.2.3", "v1.2.3"},                // contains dot, leave alone
		{"abc1234", "abc1234"},               // short, no dot
		{"abc1234567890abcdef", "abc1234"},   // long, no dot, truncate to 7
	}
	for _, c := range cases {
		if got := shortRevision(c.in); got != c.want {
			t.Errorf("shortRevision(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHumanizeAge(t *testing.T) {
	now := time.Now()
	cases := []struct {
		name string
		in   time.Time
		want string
	}{
		{"zero", time.Time{}, "—"},
		{"seconds", now.Add(-30 * time.Second), "30s ago"},
		{"minutes", now.Add(-5 * time.Minute), "5m ago"},
		{"hours", now.Add(-2 * time.Hour), "2h ago"},
		{"days", now.Add(-3 * 24 * time.Hour), "3d ago"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := humanizeAge(c.in); got != c.want {
				t.Errorf("humanizeAge = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDeriveAppStatus(t *testing.T) {
	now := metav1.Now()
	cases := []struct {
		name       string
		suspended  bool
		conds      []metav1.Condition
		wantStatus string
		wantSync   string
	}{
		{"suspended wins", true, []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue}}, "paused", "Suspended"},
		{"no conditions", false, nil, "progressing", "Progressing"},
		{"ready true", false, []metav1.Condition{{Type: "Ready", Status: metav1.ConditionTrue, LastTransitionTime: now}}, "healthy", "Synced"},
		{"ready false", false, []metav1.Condition{{Type: "Ready", Status: metav1.ConditionFalse, LastTransitionTime: now, Message: "boom"}}, "failing", "OutOfSync"},
		{"reconciling beats ready", false, []metav1.Condition{
			{Type: "Reconciling", Status: metav1.ConditionTrue, LastTransitionTime: now, Message: "applying"},
			{Type: "Ready", Status: metav1.ConditionFalse, LastTransitionTime: now},
		}, "progressing", "Progressing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, sync, _ := deriveAppStatus(c.suspended, c.conds)
			if status != c.wantStatus || sync != c.wantSync {
				t.Errorf("got status=%q sync=%q, want status=%q sync=%q", status, sync, c.wantStatus, c.wantSync)
			}
		})
	}
}

func TestAppID(t *testing.T) {
	got := appID("prod-eu-west", "shop", "Kustomization", "checkout-api")
	want := "prod-eu-west/shop/Kustomization/checkout-api"
	if got != want {
		t.Errorf("appID = %q, want %q", got, want)
	}
}

func TestLastReconcileTime(t *testing.T) {
	t1 := metav1.NewTime(time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC))
	t2 := metav1.NewTime(time.Date(2026, 5, 1, 11, 0, 0, 0, time.UTC))
	got := lastReconcileTime([]metav1.Condition{
		{Type: "Ready", LastTransitionTime: t1},
		{Type: "Reconciling", LastTransitionTime: t2},
	})
	if !got.Equal(t2.Time) {
		t.Errorf("lastReconcileTime = %v, want %v", got, t2.Time)
	}
}

// ---------- handler tests ----------

func TestApplicationsHandler_NilRegistry(t *testing.T) {
	h := &applicationsHandler{registry: nil}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	w := httptest.NewRecorder()

	h.list(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp types.ApplicationsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Applications) != 0 {
		t.Errorf("expected empty applications, got %d", len(resp.Applications))
	}
}

func TestApplicationsHandler_AggregatesAndSorts(t *testing.T) {
	now := metav1.Now()

	clusterA := newTestEntry("alpha", "Alpha Cluster",
		mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now),
	)
	clusterB := newTestEntry("bravo", "Bravo Cluster",
		mkKustomization("auth", "identity", false, false, "main@a8e2f10", now),
		mkHelmRelease("monitoring", "observability", false, true, "58.3.0", now),
	)

	// Pass entries out of natural order to verify sort.
	reg := &stubRegistry{entries: []*cluster.Entry{clusterB, clusterA}}
	h := &applicationsHandler{registry: reg}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp types.ApplicationsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Applications) != 3 {
		t.Fatalf("got %d apps, want 3", len(resp.Applications))
	}

	// Sorted by cluster (display name) then ns then name.
	wantOrder := []struct{ cluster, ns, name string }{
		{"Alpha Cluster", "shop", "checkout-api"},
		{"Bravo Cluster", "identity", "auth"},
		{"Bravo Cluster", "observability", "monitoring"},
	}
	for i, want := range wantOrder {
		got := resp.Applications[i]
		if got.Cluster != want.cluster || got.Ns != want.ns || got.Name != want.name {
			t.Errorf("app[%d] = %+v, want %+v", i, got, want)
		}
	}
}

func TestApplicationsHandler_FilterByCluster(t *testing.T) {
	now := metav1.Now()

	clusterA := newTestEntry("alpha", "Alpha Cluster",
		mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now),
	)
	clusterB := newTestEntry("bravo", "Bravo Cluster",
		mkHelmRelease("monitoring", "observability", false, true, "58.3.0", now),
	)

	reg := &stubRegistry{entries: []*cluster.Entry{clusterA, clusterB}}
	h := &applicationsHandler{registry: reg}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications?cluster=bravo", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	var resp types.ApplicationsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Applications) != 1 {
		t.Fatalf("got %d apps, want 1 (bravo only)", len(resp.Applications))
	}
	if resp.Applications[0].ClusterID != "bravo" {
		t.Errorf("clusterId = %q, want bravo", resp.Applications[0].ClusterID)
	}
}

func TestApplicationsHandler_FiltersByPolicy(t *testing.T) {
	now := metav1.Now()

	clusterA := newTestEntry("alpha", "Alpha Cluster",
		mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now),
	)
	clusterB := newTestEntry("bravo", "Bravo Cluster",
		mkHelmRelease("monitoring", "observability", false, true, "58.3.0", now),
	)

	policy := auth.Policy{
		DefaultAction: auth.ActionDeny,
		Rules: []auth.Rule{
			{Subjects: []string{"group:alpha-only"}, Verbs: []string{"get"}, Clusters: []string{"alpha"}, Action: auth.ActionAllow},
		},
	}
	id := auth.Identity{Subject: "u1", Groups: []string{"alpha-only"}}

	reg := &stubRegistry{entries: []*cluster.Entry{clusterA, clusterB}}
	h := &applicationsHandler{registry: reg, policy: policy}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	req = req.WithContext(auth.WithIdentity(req.Context(), id))
	w := httptest.NewRecorder()
	h.list(w, req)

	var resp types.ApplicationsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Applications) != 1 {
		t.Fatalf("got %d apps, want 1 (alpha only)", len(resp.Applications))
	}
	if resp.Applications[0].ClusterID != "alpha" {
		t.Errorf("clusterId = %q, want alpha", resp.Applications[0].ClusterID)
	}
}

func TestApplicationsHandler_PartialFailure(t *testing.T) {
	now := metav1.Now()

	healthy := newTestEntry("ok-cluster", "OK Cluster",
		mkKustomization("checkout-api", "shop", false, true, "main@7f3c1d9", now),
	)
	// Mark unreachable so listApplicationsForCluster returns an error.
	unreachable := newTestEntry("down-cluster", "Down Cluster")
	unreachable.SetStatus(cluster.Status{Reachable: false})

	reg := &stubRegistry{entries: []*cluster.Entry{healthy, unreachable}}
	h := &applicationsHandler{registry: reg}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/applications", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	var resp types.ApplicationsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Applications) != 1 {
		t.Fatalf("got %d apps, want 1 (only the reachable cluster)", len(resp.Applications))
	}
	if len(resp.Errors) != 1 {
		t.Fatalf("got %d errors, want 1", len(resp.Errors))
	}
	if resp.Errors[0].Cluster != "down-cluster" {
		t.Errorf("error cluster = %q, want down-cluster", resp.Errors[0].Cluster)
	}
}

// ---------- test helpers ----------

type stubRegistry struct {
	entries []*cluster.Entry
}

func (s *stubRegistry) List() []*cluster.Entry { return s.entries }
func (s *stubRegistry) Get(name string) (*cluster.Entry, bool) {
	for _, e := range s.entries {
		if e.Name == name {
			return e, true
		}
	}
	return nil, false
}

// newTestEntry builds a cluster.Entry with a fake controller-runtime client
// seeded with the given Flux objects. The entry is marked Reachable +
// FluxInstalled so handlers fan out to it.
func newTestEntry(id, displayName string, objs ...client.Object) *cluster.Entry {
	builder := fake.NewClientBuilder().
		WithScheme(cluster.RemoteScheme()).
		WithStatusSubresource(&kustomizev1.Kustomization{}, &helmv2.HelmRelease{})
	if len(objs) > 0 {
		builder = builder.WithObjects(objs...)
	}
	e := &cluster.Entry{
		Name:          id,
		DisplayName:   displayName,
		FluxNamespace: "flux-system",
		Client:        builder.Build(),
		Discovery:     &fakeDiscovery{info: &version.Info{GitVersion: "v1.30.0"}},
	}
	e.SetStatus(cluster.Status{
		Reachable:         true,
		FluxInstalled:     true,
		KubernetesVersion: "v1.30.0",
	})
	return e
}

func mkKustomization(name, ns string, suspended, ready bool, revision string, now metav1.Time) *kustomizev1.Kustomization {
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	return &kustomizev1.Kustomization{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: kustomizev1.KustomizationSpec{
			Suspend: suspended,
			SourceRef: kustomizev1.CrossNamespaceSourceReference{
				Kind: "GitRepository",
				Name: ns + "-services",
			},
		},
		Status: kustomizev1.KustomizationStatus{
			LastAppliedRevision: revision,
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "applied"},
			},
		},
	}
}

func mkHelmRelease(name, ns string, suspended, ready bool, revision string, now metav1.Time) *helmv2.HelmRelease {
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	return &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: helmv2.HelmReleaseSpec{
			Suspend: suspended,
			ChartRef: &helmv2.CrossNamespaceSourceReference{
				Kind: "HelmRepository",
				Name: ns + "-charts",
			},
		},
		Status: helmv2.HelmReleaseStatus{
			LastAttemptedRevision: revision,
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "released"},
			},
		},
	}
}

// fakeDiscovery is a discovery.DiscoveryInterface stub for tests.
type fakeDiscovery struct {
	discovery.DiscoveryInterface
	info *version.Info
	err  error
}

func (f *fakeDiscovery) ServerVersion() (*version.Info, error) { return f.info, f.err }
