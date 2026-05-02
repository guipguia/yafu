package web

import "net/http"

// Register mounts the embedded UI as the catch-all at /. API and probe
// routes registered with more specific patterns take precedence (Go 1.22
// ServeMux specificity). The handler itself rejects non-GET methods.
func Register(mux *http.ServeMux) {
	mux.Handle("/", handler())
}
