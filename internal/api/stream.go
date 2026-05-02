package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/watch"
)

// streamHandler serves the long-lived SSE connection at /api/v1/stream.
// Each connected client subscribes to the watch.Hub and receives one
// "invalidate" event per Kubernetes change on a cluster they're
// allowed to read. Heartbeats keep proxies and load balancers from
// dropping idle connections.
type streamHandler struct {
	hub    *watch.Hub
	policy auth.Policy
}

// serve writes Server-Sent Events. The handler returns when the
// client disconnects (r.Context cancellation) or the server shuts
// down. Subscribers that fall behind (full buffer) silently drop
// events — clients fall back to TanStack Query's polling cadence.
func (h *streamHandler) serve(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Tell the client we're alive before any informers fire.
	fmt.Fprintf(w, "event: hello\ndata: {}\n\n")
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	id, _ := auth.IdentityFrom(r.Context())

	var events <-chan watch.Event
	if h.hub != nil {
		ch, unsub := h.hub.Subscribe()
		defer unsub()
		events = ch
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-events:
			if !ok {
				return
			}
			// Filter by RBAC: a user that can't read the cluster
			// shouldn't even know its resources are changing.
			// Same verb the list endpoints use ("get") — a user
			// who can list /api/v1/applications?cluster=X should
			// also see invalidations for cluster X.
			if !h.policy.Authorize(id, "get", ev.Cluster) {
				continue
			}
			b, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: invalidate\ndata: %s\n\n", b)
			flusher.Flush()
		case t := <-heartbeat.C:
			fmt.Fprintf(w, ": ping %d\n\n", t.Unix())
			flusher.Flush()
		}
	}
}
