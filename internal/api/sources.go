package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

type sourcesHandler struct {
	registry cluster.Registry
	policy   auth.Policy
}

func (h *sourcesHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := types.SourcesResponse{Sources: []types.Source{}}

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
		sources []types.Source
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
			srcs, err := listSourcesForCluster(ctx, e)
			if err != nil {
				results <- result{err: &types.ClusterError{Cluster: e.Name, Error: err.Error()}}
				return
			}
			results <- result{sources: srcs}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	for res := range results {
		if res.err != nil {
			resp.Errors = append(resp.Errors, *res.err)
			continue
		}
		resp.Sources = append(resp.Sources, res.sources...)
	}

	sort.Slice(resp.Sources, func(i, j int) bool {
		a, b := resp.Sources[i], resp.Sources[j]
		if a.Cluster != b.Cluster {
			return a.Cluster < b.Cluster
		}
		if a.Ns != b.Ns {
			return a.Ns < b.Ns
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})

	_ = json.NewEncoder(w).Encode(resp)
}

func listSourcesForCluster(ctx context.Context, e *cluster.Entry) ([]types.Source, error) {
	if !e.Status().Reachable {
		return nil, fmt.Errorf("cluster unreachable")
	}
	if !e.Status().FluxInstalled {
		return nil, fmt.Errorf("flux not installed")
	}

	out := []types.Source{}

	var gits sourcev1.GitRepositoryList
	if err := e.Client.List(ctx, &gits); err == nil {
		for i := range gits.Items {
			out = append(out, gitRepoToSource(e, &gits.Items[i]))
		}
	}

	var helms sourcev1.HelmRepositoryList
	if err := e.Client.List(ctx, &helms); err == nil {
		for i := range helms.Items {
			out = append(out, helmRepoToSource(e, &helms.Items[i]))
		}
	}

	var ocis sourcev1.OCIRepositoryList
	if err := e.Client.List(ctx, &ocis); err == nil {
		for i := range ocis.Items {
			out = append(out, ociRepoToSource(e, &ocis.Items[i]))
		}
	}

	var buckets sourcev1.BucketList
	if err := e.Client.List(ctx, &buckets); err == nil {
		for i := range buckets.Items {
			out = append(out, bucketToSource(e, &buckets.Items[i]))
		}
	}

	return out, nil
}

func gitRepoToSource(e *cluster.Entry, g *sourcev1.GitRepository) types.Source {
	rev := ""
	if g.Status.Artifact != nil {
		rev = shortRevision(g.Status.Artifact.Revision)
	}
	status := sourceStatus(g.Status.Conditions)
	if g.Spec.Suspend {
		status = "paused"
	}
	return types.Source{
		ID:        sourceID(e.Name, g.Namespace, "GitRepository", g.Name),
		Name:      g.Name,
		Kind:      "GitRepository",
		Ns:        g.Namespace,
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		URL:       g.Spec.URL,
		Ref:       gitRepoRef(g.Spec.Reference),
		Revision:  rev,
		Status:    status,
		Interval:  g.Spec.Interval.Duration.String(),
		Age:       humanizeAge(lastReconcileTime(g.Status.Conditions)),
		Message:   sourceMessage(g.Status.Conditions),
		Suspended: g.Spec.Suspend,
	}
}

func helmRepoToSource(e *cluster.Entry, h *sourcev1.HelmRepository) types.Source {
	rev := ""
	if h.Status.Artifact != nil {
		rev = shortRevision(h.Status.Artifact.Revision)
	}
	status := sourceStatus(h.Status.Conditions)
	if h.Spec.Suspend {
		status = "paused"
	}
	return types.Source{
		ID:        sourceID(e.Name, h.Namespace, "HelmRepository", h.Name),
		Name:      h.Name,
		Kind:      "HelmRepository",
		Ns:        h.Namespace,
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		URL:       h.Spec.URL,
		Ref:       "—",
		Revision:  rev,
		Status:    status,
		Interval:  h.Spec.Interval.Duration.String(),
		Age:       humanizeAge(lastReconcileTime(h.Status.Conditions)),
		Message:   sourceMessage(h.Status.Conditions),
		Suspended: h.Spec.Suspend,
	}
}

func ociRepoToSource(e *cluster.Entry, o *sourcev1.OCIRepository) types.Source {
	rev := ""
	if o.Status.Artifact != nil {
		rev = shortRevision(o.Status.Artifact.Revision)
	}
	status := sourceStatus(o.Status.Conditions)
	if o.Spec.Suspend {
		status = "paused"
	}
	return types.Source{
		ID:        sourceID(e.Name, o.Namespace, "OCIRepository", o.Name),
		Name:      o.Name,
		Kind:      "OCIRepository",
		Ns:        o.Namespace,
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		URL:       o.Spec.URL,
		Ref:       ociRepoRef(o.Spec.Reference),
		Revision:  rev,
		Status:    status,
		Interval:  o.Spec.Interval.Duration.String(),
		Age:       humanizeAge(lastReconcileTime(o.Status.Conditions)),
		Message:   sourceMessage(o.Status.Conditions),
		Suspended: o.Spec.Suspend,
	}
}

func bucketToSource(e *cluster.Entry, b *sourcev1.Bucket) types.Source {
	rev := ""
	if b.Status.Artifact != nil {
		rev = shortRevision(b.Status.Artifact.Revision)
	}
	status := sourceStatus(b.Status.Conditions)
	if b.Spec.Suspend {
		status = "paused"
	}
	return types.Source{
		ID:        sourceID(e.Name, b.Namespace, "Bucket", b.Name),
		Name:      b.Name,
		Kind:      "Bucket",
		Ns:        b.Namespace,
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		URL:       fmt.Sprintf("%s/%s", b.Spec.Endpoint, b.Spec.BucketName),
		Ref:       "—",
		Revision:  rev,
		Status:    status,
		Interval:  b.Spec.Interval.Duration.String(),
		Age:       humanizeAge(lastReconcileTime(b.Status.Conditions)),
		Message:   sourceMessage(b.Status.Conditions),
		Suspended: b.Spec.Suspend,
	}
}

func gitRepoRef(ref *sourcev1.GitRepositoryRef) string {
	if ref == nil {
		return ""
	}
	switch {
	case ref.Commit != "":
		return "commit:" + shortRevision(ref.Commit)
	case ref.SemVer != "":
		return "semver:" + ref.SemVer
	case ref.Tag != "":
		return "tag:" + ref.Tag
	case ref.Name != "":
		return ref.Name
	case ref.Branch != "":
		return "branch:" + ref.Branch
	default:
		return ""
	}
}

func ociRepoRef(ref *sourcev1.OCIRepositoryRef) string {
	if ref == nil {
		return ""
	}
	switch {
	case ref.Digest != "":
		return "digest:" + shortRevision(ref.Digest)
	case ref.SemVer != "":
		return "semver:" + ref.SemVer
	case ref.Tag != "":
		return "tag:" + ref.Tag
	default:
		return ""
	}
}

func sourceStatus(conds []metav1.Condition) string {
	ready := findCondition(conds, "Ready")
	if ready == nil {
		return "progressing"
	}
	if ready.Status == metav1.ConditionTrue {
		return "healthy"
	}
	return "failing"
}

func sourceMessage(conds []metav1.Condition) string {
	if c := findCondition(conds, "Ready"); c != nil {
		return c.Message
	}
	return ""
}

func sourceID(cluster, ns, kind, name string) string {
	return cluster + "/" + ns + "/" + kind + "/" + name
}
