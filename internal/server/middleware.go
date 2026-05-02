package server

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/guipguia/yafu/internal/metrics"
)

type middleware func(http.Handler) http.Handler

func chain(h http.Handler, mws ...middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type ctxKey int

const requestIDKey ctxKey = iota

// RequestID returns the request id attached to ctx, or "" when absent.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// withRequestID echoes an incoming X-Request-ID header (if present) or
// generates a new UUID, then stashes it in the request context and the
// response headers so client + log + metrics share one identifier.
func withRequestID() middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get("X-Request-ID")
			if id == "" {
				id = uuid.NewString()
			}
			w.Header().Set("X-Request-ID", id)
			ctx := context.WithValue(r.Context(), requestIDKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// withObservability records HTTP metrics + access logs in a single
// middleware so we share one statusRecorder rather than nesting two.
func withObservability(logger *slog.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			elapsed := time.Since(start)
			statusStr := strconv.Itoa(rec.status)
			metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
			metrics.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(elapsed.Seconds())

			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", elapsed.Milliseconds(),
				"request_id", RequestID(r.Context()),
			)
		})
	}
}

func withRecover(logger *slog.Logger) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic in handler",
						"panic", rec,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
						"request_id", RequestID(r.Context()),
					)
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
