package api

import (
	"net/http"

	"github.com/guipguia/yafu/internal/cluster"
)

// Deps are the runtime dependencies injected into the HTTP API.
type Deps struct {
	Registry cluster.Registry
}

// Register mounts API routes on mux.
func Register(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleReadyz)
	mux.HandleFunc("GET /api/v1/version", handleVersion)
	mux.HandleFunc("GET /api/v1/stream", handleStream)

	ch := &clustersHandler{registry: deps.Registry}
	mux.HandleFunc("GET /api/v1/clusters", ch.list)

	ah := &applicationsHandler{registry: deps.Registry}
	mux.HandleFunc("GET /api/v1/applications", ah.list)
}
