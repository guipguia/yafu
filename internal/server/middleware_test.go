package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_NoMiddleware(t *testing.T) {
	if got := RequestID(context.Background()); got != "" {
		t.Errorf("RequestID without middleware = %q, want empty", got)
	}
}

func TestWithRequestID_GeneratesUUID(t *testing.T) {
	var captured string
	h := withRequestID()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = RequestID(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if captured == "" {
		t.Fatal("expected generated request ID in context")
	}
	if got := w.Header().Get("X-Request-ID"); got != captured {
		t.Errorf("response header X-Request-ID = %q, want %q (matches context)", got, captured)
	}
	// UUIDv4 string is 36 characters with hyphens.
	if len(captured) != 36 {
		t.Errorf("expected UUID-shaped id (36 chars), got %d: %q", len(captured), captured)
	}
}

func TestWithRequestID_PassesIncomingHeader(t *testing.T) {
	const incoming = "trace-from-client-abc123"
	var captured string
	h := withRequestID()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = RequestID(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-ID", incoming)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if captured != incoming {
		t.Errorf("RequestID = %q, want %q (echoed from header)", captured, incoming)
	}
	if got := w.Header().Get("X-Request-ID"); got != incoming {
		t.Errorf("response header = %q, want %q", got, incoming)
	}
}

func TestStatusRecorder(t *testing.T) {
	rec := httptest.NewRecorder()
	sr := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}

	sr.WriteHeader(http.StatusTeapot)

	if sr.status != http.StatusTeapot {
		t.Errorf("recorder status = %d, want %d", sr.status, http.StatusTeapot)
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("delegate status = %d, want %d (passed through)", rec.Code, http.StatusTeapot)
	}
}
