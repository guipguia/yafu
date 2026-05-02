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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

// maxEventsPerCluster bounds per-cluster event payload before fan-out
// merge. Active clusters can produce hundreds of events; a hard cap
// keeps responses snappy until informers + SSE land.
const maxEventsPerCluster = 200

type eventsHandler struct {
	registry cluster.Registry
	policy   auth.Policy
}

// eventFilter narrows /api/v1/events to events involving a specific
// resource. Empty fields match anything; the AppDetail Events tab sets
// all four to scope to one application.
type eventFilter struct {
	cluster string // matches Entry.Name
	ns      string // matches involvedObject.namespace
	kind    string // matches involvedObject.kind
	name    string // matches involvedObject.name
}

func (f eventFilter) matchesObject(ev *corev1.Event) bool {
	if f.ns != "" && ev.InvolvedObject.Namespace != f.ns {
		return false
	}
	if f.kind != "" && ev.InvolvedObject.Kind != f.kind {
		return false
	}
	if f.name != "" && ev.InvolvedObject.Name != f.name {
		return false
	}
	return true
}

func (h *eventsHandler) list(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := types.EventsResponse{Events: []types.Event{}}

	if h.registry == nil {
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	id, _ := auth.IdentityFrom(r.Context())

	q := r.URL.Query()
	filter := eventFilter{
		cluster: q.Get("cluster"),
		ns:      q.Get("ns"),
		kind:    q.Get("kind"),
		name:    q.Get("name"),
	}

	allEntries := h.registry.List()
	entries := allEntries[:0]
	for _, e := range allEntries {
		if filter.cluster != "" && e.Name != filter.cluster {
			continue
		}
		if !h.policy.Authorize(id, "get", e.Name) {
			continue
		}
		entries = append(entries, e)
	}

	type result struct {
		events []types.Event
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
			evs, err := listEventsForCluster(ctx, e, filter)
			if err != nil {
				results <- result{err: &types.ClusterError{Cluster: e.Name, Error: err.Error()}}
				return
			}
			results <- result{events: evs}
		}()
	}
	go func() { wg.Wait(); close(results) }()

	for res := range results {
		if res.err != nil {
			resp.Errors = append(resp.Errors, *res.err)
			continue
		}
		resp.Events = append(resp.Events, res.events...)
	}

	// Newest first.
	sort.Slice(resp.Events, func(i, j int) bool { return resp.Events[i].T > resp.Events[j].T })
	if len(resp.Events) > maxEventsPerCluster*4 {
		resp.Events = resp.Events[:maxEventsPerCluster*4]
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func listEventsForCluster(ctx context.Context, e *cluster.Entry, filter eventFilter) ([]types.Event, error) {
	if !e.Status().Reachable {
		return nil, fmt.Errorf("cluster unreachable")
	}

	listOpts := []client.ListOption{&client.ListOptions{Limit: maxEventsPerCluster * 5}}
	if filter.ns != "" {
		listOpts = append(listOpts, client.InNamespace(filter.ns))
	}

	var list corev1.EventList
	if err := e.Client.List(ctx, &list, listOpts...); err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}

	out := make([]types.Event, 0, len(list.Items))
	for i := range list.Items {
		ev := &list.Items[i]
		if !isFluxEvent(ev) {
			continue
		}
		if !filter.matchesObject(ev) {
			continue
		}
		out = append(out, eventToDTO(e, ev))
	}

	// Newest first per-cluster, then truncate.
	sort.Slice(out, func(i, j int) bool { return out[i].T > out[j].T })
	if len(out) > maxEventsPerCluster {
		out = out[:maxEventsPerCluster]
	}
	return out, nil
}

func isFluxEvent(ev *corev1.Event) bool {
	return strings.Contains(ev.InvolvedObject.APIVersion, "fluxcd.io")
}

func eventToDTO(e *cluster.Entry, ev *corev1.Event) types.Event {
	t := eventTimestamp(ev)
	kind := "ok"
	switch ev.Type {
	case corev1.EventTypeWarning:
		kind = "err"
	case "Info":
		kind = "ok"
	default:
		kind = "ok"
	}

	return types.Event{
		ID:        ev.Namespace + "/" + ev.Name,
		T:         t.UTC().Format(time.RFC3339),
		Cluster:   e.DisplayName,
		ClusterID: e.Name,
		Ns:        ev.Namespace,
		Kind:      kind,
		Type:      ev.Type,
		Reason:    ev.Reason,
		Message:   ev.Message,
		Object:    ev.InvolvedObject.Kind + "/" + ev.InvolvedObject.Name,
		Source:    ev.Source.Component,
	}
}

// eventTimestamp prefers LastTimestamp, falling back to EventTime then
// CreationTimestamp — k8s emitters set these inconsistently across
// controllers and versions.
func eventTimestamp(ev *corev1.Event) time.Time {
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	if !ev.EventTime.IsZero() {
		return ev.EventTime.Time
	}
	return ev.CreationTimestamp.Time
}
