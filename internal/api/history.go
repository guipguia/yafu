package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
)

func (h *applicationsHandler) history(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	resp := types.AppHistoryResponse{
		AppID:   appID(clusterID, ns, kind, name),
		Entries: []types.AppHistoryEntry{},
	}

	switch kind {
	case "HelmRelease":
		obj, err := getApp(ctx, e.Client, ns, kind, name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		hr := obj.(*helmv2.HelmRelease)
		resp.Entries = helmReleaseHistory(hr)

	case "Kustomization":
		obj, err := getApp(ctx, e.Client, ns, kind, name)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		k := obj.(*kustomizev1.Kustomization)
		resp.Entries = kustomizationHistory(k)
		resp.Note = "Kustomizations only retain their current revision; full history requires scrubbing Events (v0.3)."

	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported kind %q", kind))
		return
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func helmReleaseHistory(hr *helmv2.HelmRelease) []types.AppHistoryEntry {
	if len(hr.Status.History) == 0 {
		return []types.AppHistoryEntry{}
	}
	// History is newest-first by Flux convention; the latest deployed snapshot is current.
	out := make([]types.AppHistoryEntry, 0, len(hr.Status.History))
	for i, snap := range hr.Status.History {
		if snap == nil {
			continue
		}
		entry := types.AppHistoryEntry{
			Revision:   fmt.Sprintf("%s · v%d", snap.ChartVersion, snap.Version),
			Status:     snap.Status,
			Action:     string(snap.Action),
			AppVersion: snap.AppVersion,
			Current:    i == 0 && snap.Status == "deployed",
		}
		if !snap.LastDeployed.IsZero() {
			entry.Timestamp = snap.LastDeployed.UTC().Format(time.RFC3339)
		} else if !snap.FirstDeployed.IsZero() {
			entry.Timestamp = snap.FirstDeployed.UTC().Format(time.RFC3339)
		}
		out = append(out, entry)
	}
	return out
}

func kustomizationHistory(k *kustomizev1.Kustomization) []types.AppHistoryEntry {
	if k.Status.LastAppliedRevision == "" {
		return []types.AppHistoryEntry{}
	}
	entry := types.AppHistoryEntry{
		Revision: shortRevision(k.Status.LastAppliedRevision),
		Status:   "applied",
		Current:  true,
	}
	if t := lastReconcileTime(k.Status.Conditions); !t.IsZero() {
		entry.Timestamp = t.UTC().Format(time.RFC3339)
	}
	return []types.AppHistoryEntry{entry}
}
