package api

import (
	"encoding/json"
	"net/http"
)

// render is the stub for the rendered Git-vs-cluster diff endpoint.
// It exists so the route is reachable from the frontend and returns
// a structured 501 instead of an opaque 404 from the ServeMux.
//
// The real implementation pulls the source-controller artifact for
// the application's source ref, runs `kustomize build` (or
// `helm template`) against the resolved revision, and diffs each
// rendered resource against the live cluster state. That work lands
// in a follow-up commit (true Git-vs-cluster diff). Until then this
// stub is the contract.
func (h *applicationsHandler) render(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "rendered Git-vs-cluster diff is not yet implemented; the Drift sub-tab is the working diff today",
		"hint":  "this endpoint will return source render + per-resource diff once kustomize build / helm template integration lands",
	})
}
