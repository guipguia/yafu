package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	imagereflv1 "github.com/fluxcd/image-reflector-controller/api/v1"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

func TestImageUpdatesHandler_JoinsRepo(t *testing.T) {
	now := metav1.Now()
	objs := []client.Object{
		mkImageRepo("checkout", "flux-system", "ghcr.io/acme/checkout", now),
		mkImagePolicy("checkout-semver", "flux-system", "checkout", semverPolicy("~1.x"), "1.8.1", true, now),
		mkImagePolicy("frontend-alpha", "flux-system", "missing-repo", alphabeticalPolicy(), "", true, now),
	}
	e := newImagesEntry("alpha", "Alpha", objs...)

	reg := &stubRegistry{entries: []*cluster.Entry{e}}
	h := &imageUpdatesHandler{registry: reg, policy: auth.DefaultAllowAllPolicy}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/image-updates", nil)
	w := httptest.NewRecorder()
	h.list(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", w.Code, w.Body.String())
	}
	var resp types.ImageUpdatesResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Updates) != 2 {
		t.Fatalf("got %d updates, want 2", len(resp.Updates))
	}

	by := map[string]types.ImageUpdate{}
	for _, u := range resp.Updates {
		by[u.Name] = u
	}
	if got := by["checkout-semver"]; got.Image != "ghcr.io/acme/checkout" || got.LatestTag != "1.8.1" {
		t.Errorf("checkout-semver wrong: %+v", got)
	}
	if got := by["checkout-semver"]; got.Policy != "semver:~1.x" {
		t.Errorf("policy label = %q", got.Policy)
	}
	if got := by["frontend-alpha"]; got.Image != "" {
		t.Errorf("orphan policy should have empty Image, got %q", got.Image)
	}
	if got := by["frontend-alpha"]; got.Policy != "alphabetical" {
		t.Errorf("alphabetical label = %q", got.Policy)
	}
}

func TestPolicyChoiceLabel(t *testing.T) {
	cases := []struct {
		name   string
		choice imagereflv1.ImagePolicyChoice
		want   string
	}{
		{"semver", semverPolicy("~1.x"), "semver:~1.x"},
		{"alphabetical", alphabeticalPolicy(), "alphabetical"},
		{"numerical", numericalPolicy(), "numerical"},
		{"empty", imagereflv1.ImagePolicyChoice{}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := policyChoiceLabel(c.choice); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

// ---------- fixtures ----------

func newImagesEntry(id, displayName string, objs ...client.Object) *cluster.Entry {
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

func mkImageRepo(name, ns, image string, now metav1.Time) *imagereflv1.ImageRepository {
	return &imagereflv1.ImageRepository{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: imagereflv1.ImageRepositorySpec{
			Image:    image,
			Interval: metav1.Duration{Duration: 5 * time.Minute},
		},
	}
}

func mkImagePolicy(name, ns, repoName string, policy imagereflv1.ImagePolicyChoice, latestTag string, ready bool, now metav1.Time) *imagereflv1.ImagePolicy {
	cond := metav1.ConditionFalse
	if ready {
		cond = metav1.ConditionTrue
	}
	p := &imagereflv1.ImagePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: imagereflv1.ImagePolicySpec{
			ImageRepositoryRef: fluxmeta.NamespacedObjectReference{Name: repoName},
			Policy:             policy,
		},
		Status: imagereflv1.ImagePolicyStatus{
			Conditions: []metav1.Condition{
				{Type: "Ready", Status: cond, LastTransitionTime: now, Reason: "X", Message: "scanned"},
			},
		},
	}
	if latestTag != "" {
		p.Status.LatestRef = &imagereflv1.ImageRef{Name: repoName, Tag: latestTag}
	}
	return p
}

func semverPolicy(rng string) imagereflv1.ImagePolicyChoice {
	return imagereflv1.ImagePolicyChoice{SemVer: &imagereflv1.SemVerPolicy{Range: rng}}
}

func alphabeticalPolicy() imagereflv1.ImagePolicyChoice {
	return imagereflv1.ImagePolicyChoice{Alphabetical: &imagereflv1.AlphabeticalPolicy{}}
}

func numericalPolicy() imagereflv1.ImagePolicyChoice {
	return imagereflv1.ImagePolicyChoice{Numerical: &imagereflv1.NumericalPolicy{}}
}
