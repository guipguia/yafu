package api

import (
	"fmt"
	"net/http"
	"time"
)

// handleStream is the SSE skeleton. The MVP fans out Kubernetes
// watch events from informers; for now it just emits heartbeats so
// clients can verify the connection.
func handleStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case t := <-heartbeat.C:
			fmt.Fprintf(w, ": ping %d\n\n", t.Unix())
			flusher.Flush()
		}
	}
}
