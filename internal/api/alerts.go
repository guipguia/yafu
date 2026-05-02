package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

type alertsHandler struct {
	registry cluster.Registry
	policy   auth.Policy
}

func (h *alertsHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := types.AlertsResponse{Alerts: []types.Alert{}}

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
		alerts []types.Alert
		err    *types.ClusterError
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
			alerts, err := listAlertsForCluster(ctx, e)
			if err != nil {
				results <- result{err: &types.ClusterError{Cluster: e.Name, Error: err.Error()}}
				return
			}
			results <- result{alerts: alerts}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	for res := range results {
		if res.err != nil {
			resp.Errors = append(resp.Errors, *res.err)
			continue
		}
		resp.Alerts = append(resp.Alerts, res.alerts...)
	}

	sort.Slice(resp.Alerts, func(i, j int) bool {
		a, b := resp.Alerts[i], resp.Alerts[j]
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

func listAlertsForCluster(ctx context.Context, e *cluster.Entry) ([]types.Alert, error) {
	if !e.Status().Reachable {
		return nil, fmt.Errorf("cluster unreachable")
	}
	if !e.Status().FluxInstalled {
		return nil, fmt.Errorf("flux not installed")
	}

	var alerts notificationv1beta3.AlertList
	if err := e.Client.List(ctx, &alerts); err != nil {
		return nil, fmt.Errorf("list Alerts: %w", err)
	}

	var providers notificationv1beta3.ProviderList
	_ = e.Client.List(ctx, &providers) // best-effort; alerts still rendered as "missing" provider

	providerIdx := make(map[string]*notificationv1beta3.Provider, len(providers.Items))
	for i := range providers.Items {
		p := &providers.Items[i]
		providerIdx[p.Namespace+"/"+p.Name] = p
	}

	out := make([]types.Alert, 0, len(alerts.Items))
	for i := range alerts.Items {
		a := &alerts.Items[i]
		key := a.Namespace + "/" + a.Spec.ProviderRef.Name
		out = append(out, alertToDTO(e, a, providerIdx[key]))
	}
	return out, nil
}

func alertToDTO(e *cluster.Entry, a *notificationv1beta3.Alert, p *notificationv1beta3.Provider) types.Alert {
	provType := "missing"
	target := ""
	if p != nil {
		provType = p.Spec.Type
		switch {
		case p.Spec.Channel != "":
			target = p.Spec.Channel
		case p.Spec.Address != "":
			target = p.Spec.Address
		}
	}
	severity := a.Spec.EventSeverity
	if severity == "" {
		severity = "info"
	}
	status := "healthy"
	if a.Spec.Suspend {
		status = "paused"
	}
	return types.Alert{
		ID:        sourceID(e.Name, a.Namespace, "Alert", a.Name),
		Name:      a.Name,
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		Ns:        a.Namespace,
		Provider:  provType,
		Severity:  severity,
		Target:    target,
		Status:    status,
		Suspended: a.Spec.Suspend,
		Age:       humanizeAge(a.CreationTimestamp.Time),
	}
}
