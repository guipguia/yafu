package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/cluster"
)

type applicationsHandler struct {
	registry cluster.Registry
}

func (h *applicationsHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := types.ApplicationsResponse{Applications: []types.Application{}}

	if h.registry == nil {
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	clusterFilter := r.URL.Query().Get("cluster")
	entries := h.registry.List()
	if clusterFilter != "" {
		filtered := entries[:0]
		for _, e := range entries {
			if e.Name == clusterFilter {
				filtered = append(filtered, e)
			}
		}
		entries = filtered
	}

	type result struct {
		apps []types.Application
		err  *types.ClusterError
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
			apps, err := listApplicationsForCluster(ctx, e)
			if err != nil {
				results <- result{err: &types.ClusterError{Cluster: e.Name, Error: err.Error()}}
				return
			}
			results <- result{apps: apps}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	for res := range results {
		if res.err != nil {
			resp.Errors = append(resp.Errors, *res.err)
			continue
		}
		resp.Applications = append(resp.Applications, res.apps...)
	}

	sort.Slice(resp.Applications, func(i, j int) bool {
		ai, aj := resp.Applications[i], resp.Applications[j]
		if ai.Cluster != aj.Cluster {
			return ai.Cluster < aj.Cluster
		}
		if ai.Ns != aj.Ns {
			return ai.Ns < aj.Ns
		}
		return ai.Name < aj.Name
	})

	_ = json.NewEncoder(w).Encode(resp)
}

func listApplicationsForCluster(ctx context.Context, e *cluster.Entry) ([]types.Application, error) {
	if !e.Status().Reachable {
		return nil, fmt.Errorf("cluster unreachable")
	}
	if !e.Status().FluxInstalled {
		return nil, fmt.Errorf("flux not installed")
	}

	out := []types.Application{}

	var ks kustomizev1.KustomizationList
	if err := e.Client.List(ctx, &ks); err != nil {
		return nil, fmt.Errorf("list Kustomizations: %w", err)
	}
	for i := range ks.Items {
		out = append(out, kustomizationToApp(e, &ks.Items[i]))
	}

	var hr helmv2.HelmReleaseList
	if err := e.Client.List(ctx, &hr); err == nil {
		for i := range hr.Items {
			out = append(out, helmReleaseToApp(e, &hr.Items[i]))
		}
	}

	return out, nil
}

func kustomizationToApp(e *cluster.Entry, k *kustomizev1.Kustomization) types.Application {
	src := fmt.Sprintf("%s/%s", k.Spec.SourceRef.Kind, k.Spec.SourceRef.Name)
	if k.Spec.SourceRef.Kind == "" {
		src = k.Spec.SourceRef.Name
	}
	rev := shortRevision(k.Status.LastAppliedRevision)
	statusStr, syncStr, msg := deriveAppStatus(k.Spec.Suspend, k.Status.Conditions)

	return types.Application{
		ID:        appID(e.Name, k.Namespace, "Kustomization", k.Name),
		Name:      k.Name,
		Kind:      "Kustomization",
		Ns:        k.Namespace,
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		Status:    statusStr,
		Sync:      syncStr,
		Source:    src,
		Revision:  rev,
		Age:       humanizeAge(lastReconcileTime(k.Status.Conditions)),
		Message:   msg,
		Suspended: k.Spec.Suspend,
	}
}

func helmReleaseToApp(e *cluster.Entry, h *helmv2.HelmRelease) types.Application {
	src := helmSource(h)
	rev := h.Status.LastAttemptedRevision
	statusStr, syncStr, msg := deriveAppStatus(h.Spec.Suspend, h.Status.Conditions)

	return types.Application{
		ID:        appID(e.Name, h.Namespace, "HelmRelease", h.Name),
		Name:      h.Name,
		Kind:      "HelmRelease",
		Ns:        h.Namespace,
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		Status:    statusStr,
		Sync:      syncStr,
		Source:    src,
		Revision:  rev,
		Age:       humanizeAge(lastReconcileTime(h.Status.Conditions)),
		Message:   msg,
		Suspended: h.Spec.Suspend,
	}
}

func helmSource(h *helmv2.HelmRelease) string {
	if h.Spec.ChartRef != nil {
		return fmt.Sprintf("%s/%s", h.Spec.ChartRef.Kind, h.Spec.ChartRef.Name)
	}
	if h.Spec.Chart != nil && h.Spec.Chart.Spec.SourceRef.Name != "" {
		return fmt.Sprintf("%s/%s", h.Spec.Chart.Spec.SourceRef.Kind, h.Spec.Chart.Spec.SourceRef.Name)
	}
	return ""
}

func deriveAppStatus(suspended bool, conds []metav1.Condition) (status, sync, message string) {
	if suspended {
		return "paused", "Suspended", "Reconciliation suspended"
	}
	ready := findCondition(conds, "Ready")
	reconciling := findCondition(conds, "Reconciling")
	switch {
	case reconciling != nil && reconciling.Status == metav1.ConditionTrue:
		return "progressing", "Progressing", reconciling.Message
	case ready == nil:
		return "progressing", "Progressing", "no Ready condition reported yet"
	case ready.Status == metav1.ConditionTrue:
		return "healthy", "Synced", ready.Message
	case ready.Status == metav1.ConditionFalse:
		return "failing", "OutOfSync", ready.Message
	default:
		return "degraded", "Progressing", ready.Message
	}
}

func findCondition(conds []metav1.Condition, t string) *metav1.Condition {
	for i := range conds {
		if conds[i].Type == t {
			return &conds[i]
		}
	}
	return nil
}

func lastReconcileTime(conds []metav1.Condition) time.Time {
	var latest time.Time
	for i := range conds {
		if t := conds[i].LastTransitionTime.Time; t.After(latest) {
			latest = t
		}
	}
	return latest
}

// shortRevision strips a `branch@` prefix and truncates to 7 chars,
// matching the way the UI renders revision tokens.
func shortRevision(r string) string {
	if r == "" {
		return ""
	}
	if i := strings.LastIndex(r, "@"); i >= 0 && i+1 < len(r) {
		r = r[i+1:]
	}
	if len(r) > 12 && !strings.Contains(r, ".") {
		r = r[:7]
	}
	return r
}

// humanizeAge returns "Xs ago" / "Xm ago" / "Xh ago" / "Xd ago" relative
// to now. Empty input yields "—".
func humanizeAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours())/24)
	}
}

func appID(cluster, ns, kind, name string) string {
	return strings.Join([]string{cluster, ns, kind, name}, "/")
}
