package web

import "net/http"

// Register mounts the embedded UI at GET /. API routes registered
// before this take precedence (Go 1.22 ServeMux specificity).
func Register(mux *http.ServeMux) {
	mux.Handle("GET /", handler())
}
