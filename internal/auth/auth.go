// Package auth handles authentication of HTTP requests and exposes the
// resulting Identity to downstream handlers via the request context.
//
// Three modes are supported:
//
//   - "anonymous" — every request is given a synthetic identity. Useful for
//     local development and demos. Do not use in production.
//   - "header"     — trust X-Forwarded-User / X-Forwarded-Email /
//     X-Forwarded-Groups headers from a known reverse proxy (oauth2-proxy,
//     Pomerium, Authentik, ingress-nginx auth-snippet, etc.). The proxy must
//     strip these headers from incoming requests itself; yafu trusts what
//     the proxy injects.
//   - "oidc"       — native OIDC (Dex / Keycloak / Okta / Azure AD). Not yet
//     implemented; configuring this mode returns an error at startup.
package auth

import (
	"context"
	"fmt"
	"strings"
)

// Mode is the authentication strategy yafu runs with.
type Mode string

const (
	ModeAnonymous Mode = "anonymous"
	ModeHeader    Mode = "header"
	ModeOIDC      Mode = "oidc"
)

// ParseMode validates s and returns the corresponding Mode.
func ParseMode(s string) (Mode, error) {
	switch m := Mode(s); m {
	case ModeAnonymous, ModeHeader, ModeOIDC:
		return m, nil
	default:
		return "", fmt.Errorf("unknown auth mode %q (want anonymous|header|oidc)", s)
	}
}

// Identity is the authenticated principal making the request.
//
// Subject is the stable, unique identifier from the IDP (or "anonymous" /
// the X-Forwarded-User value depending on mode). Groups is normalized to
// lowercase strings; the policy engine matches on these case-insensitively.
type Identity struct {
	Subject string
	Email   string
	Name    string
	Groups  []string
}

// IsAnonymous reports whether the identity was produced by ModeAnonymous.
func (i Identity) IsAnonymous() bool { return i.Subject == "anonymous" }

// HasGroup reports whether the identity is a member of g (case-insensitive).
func (i Identity) HasGroup(g string) bool {
	g = strings.ToLower(g)
	for _, have := range i.Groups {
		if strings.ToLower(have) == g {
			return true
		}
	}
	return false
}

type ctxKey int

const identityKey ctxKey = iota

// IdentityFrom returns the identity attached to ctx by the auth middleware.
// Handlers should treat a missing identity as a server bug — the middleware
// is always installed in production.
func IdentityFrom(ctx context.Context) (Identity, bool) {
	v, ok := ctx.Value(identityKey).(Identity)
	return v, ok
}

// WithIdentity returns a child context carrying id.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey, id)
}
