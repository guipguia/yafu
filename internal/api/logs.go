package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	apitypes "github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
)

// defaultLogTail is the line count returned when ?tail= isn't set.
// The same cap upper-bounds tail to keep responses predictable.
const (
	defaultLogTail = 200
	maxLogTail     = 1000
	// maxLogBytes hard-caps response size so a runaway log doesn't blow up
	// the request budget. 256 KiB is more than enough for ~1k lines.
	maxLogBytes = 256 * 1024
)

func (h *applicationsHandler) logs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	clusterID := r.PathValue("cluster")
	ns := r.PathValue("ns")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	id, _ := auth.IdentityFrom(r.Context())
	if !h.policy.Authorize(id, "get", clusterID) {
		writeError(w, http.StatusForbidden, fmt.Sprintf("identity is not allowed to get cluster %q", clusterID))
		return
	}

	if h.registry == nil {
		writeError(w, http.StatusServiceUnavailable, "registry not initialised")
		return
	}
	e, ok := h.registry.Get(clusterID)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("cluster %q not registered", clusterID))
		return
	}
	if !e.Status().Reachable {
		writeError(w, http.StatusServiceUnavailable, "cluster unreachable")
		return
	}
	if e.Kube == nil {
		writeError(w, http.StatusServiceUnavailable, "kubernetes clientset unavailable on this cluster")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp := apitypes.LogsResponse{
		AppID: appID(clusterID, ns, kind, name),
		Pods:  []apitypes.PodInfo{},
	}

	obj, err := getApp(ctx, e.Client, ns, kind, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	entries, _ := inventoryEntriesOf(ctx, e.Kube, obj)
	namespaces := uniqueInventoryNamespaces(entries)
	if len(namespaces) == 0 {
		// Fallback when inventory is unavailable (HelmRelease never installed,
		// Kustomization never reconciled, etc.) — at least try the resource's
		// own namespace.
		namespaces = []string{ns}
		resp.Note = "Inventory unavailable; listing pods in the application's own namespace as a fallback."
	}

	pods, listErr := listPodsForApp(ctx, e.Kube, namespaces, entries)
	if listErr != nil {
		writeError(w, http.StatusInternalServerError, listErr.Error())
		return
	}
	resp.Pods = pods

	q := r.URL.Query()
	wantPod := q.Get("pod")
	wantContainer := q.Get("container")
	tail := parseTail(q.Get("tail"))

	selected := pickPod(pods, wantPod)
	if selected == nil {
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	resp.Selected = selected.Ns + "/" + selected.Name

	if wantContainer == "" && len(selected.Containers) > 0 {
		wantContainer = selected.Containers[0]
	}
	resp.Container = wantContainer

	logs, truncated, err := fetchPodLogs(ctx, e.Kube, selected.Ns, selected.Name, wantContainer, tail)
	if err != nil {
		// Don't 500 the whole tab when one pod's logs are unavailable;
		// surface the error in the body so the UI can still render the
		// pod list (the user can pick another pod).
		resp.Logs = fmt.Sprintf("# could not stream logs: %s\n", err.Error())
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	resp.Logs = logs
	resp.Truncated = truncated

	_ = json.NewEncoder(w).Encode(resp)
}

func uniqueInventoryNamespaces(refs []inventoryRef) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range refs {
		if r.ns == "" {
			continue
		}
		if !seen[r.ns] {
			seen[r.ns] = true
			out = append(out, r.ns)
		}
	}
	sort.Strings(out)
	return out
}

func listPodsForApp(ctx context.Context, kube kubernetes.Interface, namespaces []string, entries []inventoryRef) ([]apitypes.PodInfo, error) {
	out := []apitypes.PodInfo{}
	now := time.Now()
	for _, ns := range namespaces {
		list, err := kube.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list pods in %s: %w", ns, err)
		}
		for i := range list.Items {
			p := &list.Items[i]
			if !podMatchesInventory(p, ns, entries) {
				continue
			}
			out = append(out, podToInfo(p, now))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Ns != out[j].Ns {
			return out[i].Ns < out[j].Ns
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// podMatchesInventory returns true when pod's name starts with one of the
// workload (Deployment / StatefulSet / DaemonSet / ReplicaSet / Pod /
// Job / CronJob) names from the inventory in the same namespace. This is
// the simplest heuristic that picks the right pods for almost every Flux
// app without needing a full owner-ref walk.
func podMatchesInventory(p *corev1.Pod, ns string, entries []inventoryRef) bool {
	if len(entries) == 0 {
		return true // no inventory to filter against
	}
	for _, ref := range entries {
		if ref.ns != ns {
			continue
		}
		switch ref.kind {
		case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob", "Pod":
			if p.Name == ref.name || strings.HasPrefix(p.Name, ref.name+"-") {
				return true
			}
		}
	}
	return false
}

func podToInfo(p *corev1.Pod, now time.Time) apitypes.PodInfo {
	containers := make([]string, 0, len(p.Spec.Containers))
	for i := range p.Spec.Containers {
		containers = append(containers, p.Spec.Containers[i].Name)
	}
	var restarts int32
	for i := range p.Status.ContainerStatuses {
		restarts += p.Status.ContainerStatuses[i].RestartCount
	}
	age := ""
	if !p.CreationTimestamp.IsZero() {
		age = humanizeAgeFrom(p.CreationTimestamp.Time, now)
	}
	return apitypes.PodInfo{
		Ns:         p.Namespace,
		Name:       p.Name,
		Phase:      string(p.Status.Phase),
		Containers: containers,
		Restarts:   restarts,
		Age:        age,
	}
}

func pickPod(pods []apitypes.PodInfo, want string) *apitypes.PodInfo {
	if len(pods) == 0 {
		return nil
	}
	if want != "" {
		for i := range pods {
			full := pods[i].Ns + "/" + pods[i].Name
			if want == full || want == pods[i].Name {
				return &pods[i]
			}
		}
		return nil
	}
	// Default: prefer a Running pod, fall back to the first.
	for i := range pods {
		if pods[i].Phase == string(corev1.PodRunning) {
			return &pods[i]
		}
	}
	return &pods[0]
}

func parseTail(s string) int64 {
	if s == "" {
		return defaultLogTail
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultLogTail
	}
	if n > maxLogTail {
		return maxLogTail
	}
	return int64(n)
}

func fetchPodLogs(ctx context.Context, kube kubernetes.Interface, ns, name, container string, tail int64) (string, bool, error) {
	tailLines := tail
	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	}
	req := kube.CoreV1().Pods(ns).GetLogs(name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", false, err
	}
	defer stream.Close()

	limited := io.LimitReader(stream, maxLogBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", false, err
	}
	if len(data) > maxLogBytes {
		return string(data[:maxLogBytes]), true, nil
	}
	return string(data), false, nil
}

func humanizeAgeFrom(start, now time.Time) string {
	d := now.Sub(start)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
}
