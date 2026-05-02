package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	imagereflv1 "github.com/fluxcd/image-reflector-controller/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

type imageUpdatesHandler struct {
	registry cluster.Registry
	policy   auth.Policy
}

func (h *imageUpdatesHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := types.ImageUpdatesResponse{Updates: []types.ImageUpdate{}}

	if h.registry == nil {
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	id, _ := auth.IdentityFrom(r.Context())

	clusterFilter := r.URL.Query().Get("cluster")
	allEntries := h.registry.List()
	entries := allEntries[:0]
	for _, e := range allEntries {
		if clusterFilter != "" && e.Name != clusterFilter {
			continue
		}
		if !h.policy.Authorize(id, "get", e.Name) {
			continue
		}
		entries = append(entries, e)
	}

	type result struct {
		updates []types.ImageUpdate
		err     *types.ClusterError
	}
	results := make(chan result, len(entries))

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for _, e := range entries {
		e := e
		wg.Add(1)
		go func() {
			defer wg.Done()
			ups, err := listImageUpdatesForCluster(ctx, e)
			if err != nil {
				results <- result{err: &types.ClusterError{Cluster: e.Name, Error: err.Error()}}
				return
			}
			results <- result{updates: ups}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	for res := range results {
		if res.err != nil {
			resp.Errors = append(resp.Errors, *res.err)
			continue
		}
		resp.Updates = append(resp.Updates, res.updates...)
	}

	sort.Slice(resp.Updates, func(i, j int) bool {
		a, b := resp.Updates[i], resp.Updates[j]
		if a.Cluster != b.Cluster {
			return a.Cluster < b.Cluster
		}
		if a.Ns != b.Ns {
			return a.Ns < b.Ns
		}
		return a.Name < b.Name
	})

	_ = json.NewEncoder(w).Encode(resp)
}

func listImageUpdatesForCluster(ctx context.Context, e *cluster.Entry) ([]types.ImageUpdate, error) {
	if !e.Status().Reachable {
		return nil, fmt.Errorf("cluster unreachable")
	}
	if !e.Status().FluxInstalled {
		return nil, fmt.Errorf("flux not installed")
	}

	var policies imagereflv1.ImagePolicyList
	if err := e.Client.List(ctx, &policies); err != nil {
		// CRDs missing → image automation isn't installed; treat as empty.
		if isImageCRDsMissing(err) {
			return []types.ImageUpdate{}, nil
		}
		return nil, fmt.Errorf("list ImagePolicies: %w", err)
	}

	var repos imagereflv1.ImageRepositoryList
	_ = e.Client.List(ctx, &repos) // best-effort; missing repos render as policy without image

	repoIdx := make(map[string]*imagereflv1.ImageRepository, len(repos.Items))
	for i := range repos.Items {
		r := &repos.Items[i]
		repoIdx[r.Namespace+"/"+r.Name] = r
	}

	out := make([]types.ImageUpdate, 0, len(policies.Items))
	for i := range policies.Items {
		p := &policies.Items[i]
		out = append(out, imagePolicyToUpdate(e, p, repoIdx))
	}
	return out, nil
}

// isImageCRDsMissing returns true when the image-reflector CRDs aren't
// installed on the target cluster (image automation is optional in Flux)
// — the standard NoMatch / NotFound / scheme-not-registered patterns the
// cluster prober already handles.
func isImageCRDsMissing(err error) bool {
	return meta.IsNoMatchError(err) || apierrors.IsNotFound(err) || runtime.IsNotRegisteredError(err)
}

func imagePolicyToUpdate(e *cluster.Entry, p *imagereflv1.ImagePolicy, repoIdx map[string]*imagereflv1.ImageRepository) types.ImageUpdate {
	repoNS := p.Spec.ImageRepositoryRef.Namespace
	if repoNS == "" {
		repoNS = p.Namespace
	}
	repo := repoIdx[repoNS+"/"+p.Spec.ImageRepositoryRef.Name]

	image := ""
	if repo != nil {
		image = repo.Spec.Image
	}

	latestTag := ""
	if p.Status.LatestRef != nil {
		latestTag = p.Status.LatestRef.Tag
	}

	return types.ImageUpdate{
		ID:        sourceID(e.Name, p.Namespace, "ImagePolicy", p.Name),
		Name:      p.Name,
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		Ns:        p.Namespace,
		Image:     image,
		LatestTag: latestTag,
		Policy:    policyChoiceLabel(p.Spec.Policy),
		Status:    sourceStatus(p.Status.Conditions),
		Age:       humanizeAge(lastReconcileTime(p.Status.Conditions)),
		Message:   sourceMessage(p.Status.Conditions),
	}
}

func policyChoiceLabel(c imagereflv1.ImagePolicyChoice) string {
	switch {
	case c.SemVer != nil:
		return "semver:" + c.SemVer.Range
	case c.Alphabetical != nil:
		return "alphabetical"
	case c.Numerical != nil:
		return "numerical"
	default:
		return ""
	}
}
