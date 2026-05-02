package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Middleware authenticates a request, attaches the resulting Identity to
// the context, and then calls next. On auth failure it writes the
// appropriate status code and does NOT call next.
type Middleware func(http.Handler) http.Handler

// AuthSet bundles everything needed to wire up an authentication mode:
// the request middleware, plus optional /auth/* handlers (only OIDC
// uses these today).
type AuthSet struct {
	Middleware      Middleware
	LoginHandler    http.HandlerFunc // GET /auth/login    (OIDC: redirect to IdP)
	CallbackHandler http.HandlerFunc // GET /auth/callback (OIDC: code exchange)
	LogoutHandler   http.HandlerFunc // GET /auth/logout
}

// New returns a Middleware-only AuthSet for modes that don't need
// additional routes (anonymous, header). For OIDC, use NewOIDC.
func New(mode Mode) (Middleware, error) {
	switch mode {
	case ModeAnonymous:
		return anonymousMiddleware(), nil
	case ModeHeader:
		return headerMiddleware(), nil
	case ModeOIDC:
		return nil, fmt.Errorf("auth mode %q requires NewOIDC(ctx, OIDCConfig) — call it directly from main", mode)
	default:
		return nil, fmt.Errorf("unknown auth mode %q", mode)
	}
}

// NewSet wraps the simple modes (anonymous, header) into an AuthSet so
// the server can treat every mode uniformly.
func NewSet(mode Mode) (*AuthSet, error) {
	mw, err := New(mode)
	if err != nil {
		return nil, err
	}
	return &AuthSet{Middleware: mw}, nil
}

// anonymousMiddleware tags every request with a single synthetic identity.
func anonymousMiddleware() Middleware {
	id := Identity{
		Subject: "anonymous",
		Email:   "anonymous@yafu.local",
		Name:    "anonymous",
		Groups:  []string{"system:anonymous"},
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

// headerMiddleware reads X-Forwarded-User (required), X-Forwarded-Email,
// X-Forwarded-Preferred-Username, and X-Forwarded-Groups (comma-separated)
// from the incoming request. Requests without X-Forwarded-User are rejected
// with 401 — the proxy is responsible for ensuring the header is present
// for authenticated traffic.
func headerMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := strings.TrimSpace(r.Header.Get("X-Forwarded-User"))
			if user == "" {
				writeUnauthorized(w, "missing X-Forwarded-User header (proxy not configured?)")
				return
			}
			id := Identity{
				Subject: user,
				Email:   strings.TrimSpace(r.Header.Get("X-Forwarded-Email")),
				Name:    strings.TrimSpace(r.Header.Get("X-Forwarded-Preferred-Username")),
				Groups:  splitGroups(r.Header.Get("X-Forwarded-Groups")),
			}
			if id.Name == "" {
				id.Name = id.Subject
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

func splitGroups(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if g := strings.TrimSpace(p); g != "" {
			out = append(out, g)
		}
	}
	return out
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
