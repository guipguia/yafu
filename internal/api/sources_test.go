package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestSourcesHandler_AggregatesAllKinds(t *testing.T) {
	now := metav1.Now()
	objs := []client.Object{
		mkGitRepo("shop-services", "flux-system", "https://github.com/acme/shop-services", "main", "main@7f3c1d9", true, now),
		mkHelmRepo("jetstack", "flux-system", "https://charts.jetstack.io", "idx-acdef12", true, now),
		mkOCIRepo("platform-charts", "flux-system", "oci://ghcr.io/acme/charts", "tag:2.x", "sha256:c0ffee...", true, now),
		mkBucket("backups", "flux-system", "s3.amazonaws.com", "yafu-backups", "rev-001", false, now),
	}
	e := newSourcesEntry("alpha", "Alpha", objs...)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &sourcesHandler{registry: reg, policy: auth.DefaultAllowAllPolicy}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp types.SourcesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Sources) != 4 {
		t.Fatalf("got %d sources, want 4 (one of each kind)", len(resp.Sources))
	}

	wantKinds := map[string]bool{"GitRepository": false, "HelmRepository": false, "OCIRepository": false, "Bucket": false}
	for _, s := range resp.Sources {
		wantKinds[s.Kind] = true
	}
	for k, found := range wantKinds {
		if !found {
			t.Errorf("missing source kind %q in response", k)
		}
	}
}

func TestSourcesHandler_StatusFromConditions(t *testing.T) {
	now := metav1.Now()
	healthy := mkGitRepo("ok", "flux-system", "https://github.com/x/y", "main", "main@abc", true, now)
	failing := mkGitRepo("bad", "flux-system", "https://github.com/x/z", "main", "", false, now)
	failing.Status.Conditions[0].Message = "dial tcp: lookup github.com: i/o timeout"

	e := newSourcesEntry("alpha", "Alpha", healthy, failing)
	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &sourcesHandler{registry: reg, policy: auth.DefaultAllowAllPolicy}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	var resp types.SourcesResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)

	got := map[string]string{}
	gotMsg := map[string]string{}
	for _, s := range resp.Sources {
		got[s.Name] = s.Status
		gotMsg[s.Name] = s.Message
	}
	if got["ok"] != "healthy" {
		t.Errorf("status[ok] = %q, want healthy", got["ok"])
	}
	if got["bad"] != "failing" {
		t.Errorf("status[bad] = %q, want failing", got["bad"])
	}
	if gotMsg["bad"] == "" {
		t.Error("expected failing source to surface its condition Message")
	}
}

func TestGitRepoRef(t *testing.T) {
	cases := []struct {
		ref  *sourcev1.GitRepositoryRef
		want string
	}{
		{nil, ""},
		{&sourcev1.GitRepositoryRef{Branch: "main"}, "branch:main"},
		{&sourcev1.GitRepositoryRef{Tag: "v1.2.3"}, "tag:v1.2.3"},
		{&sourcev1.GitRepositoryRef{SemVer: "~1.x"}, "semver:~1.x"},
		// SHA gets shortened by shortRevision (no dot, longer than 12)
		{&sourcev1.GitRepositoryRef{Commit: "abc1234567890abcdef"}, "commit:abc1234"},
		// Precedence: Commit > SemVer > Tag > Name > Branch
		{&sourcev1.GitRepositoryRef{Branch: "main", Tag: "v1", SemVer: "~1", Commit: "abc1234567890abcdef"}, "commit:abc1234"},
	}
	for _, c := range cases {
		got := gitRepoRef(c.ref)
		if got != c.want {
			t.Errorf("gitRepoRef(%+v) = %q, want %q", c.ref, got, c.want)
		}
	}
}

// ---------- fixtures ----------

func newSourcesEntry(id, displayName string, objs ...client.Object) *cluster.Entry {
	builder := fake.NewClientBuilder().
		WithScheme(cluster.RemoteScheme()).
		WithStatusSubresource(
			&sourcev1.GitRepository{},
			&sourcev1.HelmRepository{},
			&sourcev1.OCIRepository{},
			&sourcev1.Bucket{},
		)
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

func mkGitRepo(name, ns, url, branch, revision string, ready bool, now metav1.Time) *sourcev1.GitRepository {
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	g := &sourcev1.GitRepository{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: sourcev1.GitRepositorySpec{
			URL:       url,
			Interval:  metav1.Duration{Duration: time.Minute},
			Reference: &sourcev1.GitRepositoryRef{Branch: branch},
		},
		Status: sourcev1.GitRepositoryStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "stored artifact"},
			},
		},
	}
	if revision != "" {
		g.Status.Artifact = &fluxmeta.Artifact{Revision: revision}
	}
	return g
}

func mkHelmRepo(name, ns, url, revision string, ready bool, now metav1.Time) *sourcev1.HelmRepository {
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	h := &sourcev1.HelmRepository{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: sourcev1.HelmRepositorySpec{
			URL:      url,
			Interval: metav1.Duration{Duration: 5 * time.Minute},
		},
		Status: sourcev1.HelmRepositoryStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "index updated"},
			},
		},
	}
	if revision != "" {
		h.Status.Artifact = &fluxmeta.Artifact{Revision: revision}
	}
	return h
}

func mkOCIRepo(name, ns, url, tag, revision string, ready bool, now metav1.Time) *sourcev1.OCIRepository {
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	o := &sourcev1.OCIRepository{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: sourcev1.OCIRepositorySpec{
			URL:       url,
			Interval:  metav1.Duration{Duration: 2 * time.Minute},
			Reference: &sourcev1.OCIRepositoryRef{Tag: tag},
		},
		Status: sourcev1.OCIRepositoryStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "pulled artifact"},
			},
		},
	}
	if revision != "" {
		o.Status.Artifact = &fluxmeta.Artifact{Revision: revision}
	}
	return o
}

func mkBucket(name, ns, endpoint, bucketName, revision string, ready bool, now metav1.Time) *sourcev1.Bucket {
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	b := &sourcev1.Bucket{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: sourcev1.BucketSpec{
			Endpoint:   endpoint,
			BucketName: bucketName,
			Interval:   metav1.Duration{Duration: 5 * time.Minute},
		},
		Status: sourcev1.BucketStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "fetched"},
			},
		},
	}
	if revision != "" {
		b.Status.Artifact = &fluxmeta.Artifact{Revision: revision}
	}
	return b
}
