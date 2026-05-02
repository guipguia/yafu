package api

import (
	"net/http"

	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

// Deps are the runtime dependencies injected into the HTTP API.
type Deps struct {
	Registry cluster.Registry
	Policy   auth.Policy
}

// RegisterPublic mounts unauthenticated routes — kubelet probes,
// readiness, etc. /metrics is mounted by the server itself.
func RegisterPublic(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /readyz", handleReadyz)
}

// RegisterAPI mounts authenticated routes under /api/v1/*. The caller is
// responsible for fronting these with the auth middleware.
func RegisterAPI(mux *http.ServeMux, deps Deps) {
	mux.HandleFunc("GET /api/v1/version", handleVersion)
	mux.HandleFunc("GET /api/v1/whoami", handleWhoami)
	mux.HandleFunc("GET /api/v1/stream", handleStream)

	ch := &clustersHandler{registry: deps.Registry, policy: deps.Policy}
	mux.HandleFunc("GET /api/v1/clusters", ch.list)

	ah := &applicationsHandler{registry: deps.Registry, policy: deps.Policy}
	mux.HandleFunc("GET /api/v1/applications", ah.list)
	mux.HandleFunc("POST /api/v1/applications/{cluster}/{ns}/{kind}/{name}/reconcile", ah.reconcile)
	mux.HandleFunc("POST /api/v1/applications/{cluster}/{ns}/{kind}/{name}/suspend", ah.suspend)
	mux.HandleFunc("POST /api/v1/applications/{cluster}/{ns}/{kind}/{name}/resume", ah.resume)

	sh := &sourcesHandler{registry: deps.Registry, policy: deps.Policy}
	mux.HandleFunc("GET /api/v1/sources", sh.list)

	alh := &alertsHandler{registry: deps.Registry, policy: deps.Policy}
	mux.HandleFunc("GET /api/v1/alerts", alh.list)

	eh := &eventsHandler{registry: deps.Registry, policy: deps.Policy}
	mux.HandleFunc("GET /api/v1/events", eh.list)
}
