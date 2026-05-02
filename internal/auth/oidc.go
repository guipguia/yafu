package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// OIDCConfig describes how to talk to an OpenID Connect provider.
//
// Issuer must be the discovery URL (e.g. "https://accounts.google.com" or
// the Dex / Keycloak realm URL). yafu fetches "{Issuer}/.well-known/openid-configuration"
// at startup; if that fails, the binary refuses to boot rather than serve
// auth in a half-broken state.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Scopes       []string
	GroupsClaim  string

	// CookieSecret signs the short-lived state cookie used to defeat CSRF
	// on the OAuth2 callback. 32 bytes (base64 / hex) recommended; if
	// empty, a random secret is generated at startup (sessions in flight
	// won't survive a restart).
	CookieSecret []byte
	// SecureCookie should be true whenever yafu is reachable over HTTPS.
	// In dev with --auth-mode=oidc on plain HTTP, set false.
	SecureCookie bool
	// CookieDomain narrows the session cookie to a specific host. Empty =
	// host-only cookie (recommended unless you have multiple subdomains
	// behind one yafu).
	CookieDomain string
}

const (
	sessionCookieName = "yafu_session"
	stateCookieName   = "yafu_oidc_state"
	stateCookieMaxAge = 5 * 60 // 5 minutes
)

// oidcAuth bundles everything needed to verify ID tokens and run the
// authorization-code-with-PKCE handshake. Built once at startup.
type oidcAuth struct {
	cfg      OIDCConfig
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    *oauth2.Config
	groupsClaim string
}

// NewOIDC performs OIDC discovery against cfg.Issuer and returns an
// auth handler set ready to mount.
func NewOIDC(ctx context.Context, cfg OIDCConfig) (*AuthSet, error) {
	if cfg.Issuer == "" {
		return nil, fmt.Errorf("oidc: issuer is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("oidc: clientID is required")
	}
	if cfg.RedirectURL == "" {
		return nil, fmt.Errorf("oidc: redirectURL is required")
	}
	if len(cfg.Scopes) == 0 {
		cfg.Scopes = []string{oidc.ScopeOpenID, "email", "profile", "groups"}
	}
	if cfg.GroupsClaim == "" {
		cfg.GroupsClaim = "groups"
	}
	if len(cfg.CookieSecret) == 0 {
		cfg.CookieSecret = mustRandom(32)
	}

	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery against %s: %w", cfg.Issuer, err)
	}

	a := &oidcAuth{
		cfg:         cfg,
		provider:    provider,
		verifier:    provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		groupsClaim: cfg.GroupsClaim,
		oauth: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.RedirectURL,
			Scopes:       cfg.Scopes,
		},
	}

	return &AuthSet{
		Middleware:      a.middleware(),
		LoginHandler:    a.handleLogin,
		CallbackHandler: a.handleCallback,
		LogoutHandler:   a.handleLogout,
	}, nil
}

func (a *oidcAuth) middleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil {
				writeUnauthorized(w, "no session")
				return
			}
			id, err := a.verify(r.Context(), cookie.Value)
			if err != nil {
				writeUnauthorized(w, "invalid session: "+err.Error())
				return
			}
			next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), id)))
		})
	}
}

func (a *oidcAuth) verify(ctx context.Context, rawIDToken string) (Identity, error) {
	token, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, err
	}
	return a.identityFromToken(token)
}

func (a *oidcAuth) identityFromToken(token *oidc.IDToken) (Identity, error) {
	// Standard claims first.
	var std struct {
		Sub               string `json:"sub"`
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	if err := token.Claims(&std); err != nil {
		return Identity{}, fmt.Errorf("decode standard claims: %w", err)
	}

	// Groups claim is configurable — extract via raw map.
	var raw map[string]any
	if err := token.Claims(&raw); err != nil {
		return Identity{}, fmt.Errorf("decode raw claims: %w", err)
	}
	groups := stringSliceFromClaim(raw[a.groupsClaim])

	name := std.Name
	if name == "" {
		name = std.PreferredUsername
	}
	if name == "" {
		name = std.Email
	}
	if name == "" {
		name = std.Sub
	}

	return Identity{
		Subject: std.Sub,
		Email:   std.Email,
		Name:    name,
		Groups:  groups,
	}, nil
}

func stringSliceFromClaim(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		// Some IdPs (Azure AD) emit a single string; comma-split as a courtesy.
		if t == "" {
			return nil
		}
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// ---------- handlers ----------

type oidcState struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	Return   string `json:"r,omitempty"`
}

func (a *oidcAuth) handleLogin(w http.ResponseWriter, r *http.Request) {
	state := oidcState{
		State:    randomString(24),
		Nonce:    randomString(24),
		Verifier: oauth2.GenerateVerifier(),
		Return:   sanitizeReturnTo(r.URL.Query().Get("return")),
	}
	if err := a.setStateCookie(w, state); err != nil {
		http.Error(w, "could not set state cookie", http.StatusInternalServerError)
		return
	}
	authURL := a.oauth.AuthCodeURL(
		state.State,
		oauth2.S256ChallengeOption(state.Verifier),
		oidc.Nonce(state.Nonce),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (a *oidcAuth) handleCallback(w http.ResponseWriter, r *http.Request) {
	state, err := a.readStateCookie(r)
	if err != nil {
		http.Error(w, "missing or invalid state cookie", http.StatusBadRequest)
		return
	}
	a.clearStateCookie(w)

	if r.URL.Query().Get("state") != state.State {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		http.Error(w, fmt.Sprintf("idp returned error: %s — %s", errParam, desc), http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := a.oauth.Exchange(r.Context(), code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		http.Error(w, "code exchange: "+err.Error(), http.StatusBadGateway)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		http.Error(w, "id_token missing from token response", http.StatusBadGateway)
		return
	}

	idToken, err := a.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		http.Error(w, "id token verification: "+err.Error(), http.StatusBadGateway)
		return
	}
	if idToken.Nonce != state.Nonce {
		http.Error(w, "nonce mismatch", http.StatusBadRequest)
		return
	}

	a.setSessionCookie(w, rawIDToken, idToken.Expiry)

	dest := state.Return
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

func (a *oidcAuth) handleLogout(w http.ResponseWriter, r *http.Request) {
	a.clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// ---------- cookies ----------

func (a *oidcAuth) setSessionCookie(w http.ResponseWriter, rawIDToken string, expires time.Time) {
	cookie := &http.Cookie{
		Name:     sessionCookieName,
		Value:    rawIDToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.cfg.SecureCookie,
		Domain:   a.cfg.CookieDomain,
	}
	if !expires.IsZero() {
		cookie.Expires = expires
	}
	http.SetCookie(w, cookie)
}

func (a *oidcAuth) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.cfg.SecureCookie,
		Domain:   a.cfg.CookieDomain,
		MaxAge:   -1,
	})
}

func (a *oidcAuth) setStateCookie(w http.ResponseWriter, st oidcState) error {
	body, err := json.Marshal(st)
	if err != nil {
		return err
	}
	signed := signCookieValue(a.cfg.CookieSecret, body)
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookieName,
		Value:    signed,
		Path:     "/auth",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.cfg.SecureCookie,
		Domain:   a.cfg.CookieDomain,
		MaxAge:   stateCookieMaxAge,
	})
	return nil
}

func (a *oidcAuth) readStateCookie(r *http.Request) (oidcState, error) {
	cookie, err := r.Cookie(stateCookieName)
	if err != nil {
		return oidcState{}, err
	}
	body, err := verifyCookieValue(a.cfg.CookieSecret, cookie.Value)
	if err != nil {
		return oidcState{}, err
	}
	var st oidcState
	if err := json.Unmarshal(body, &st); err != nil {
		return oidcState{}, err
	}
	return st, nil
}

func (a *oidcAuth) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   stateCookieName,
		Value:  "",
		Path:   "/auth",
		MaxAge: -1,
		Domain: a.cfg.CookieDomain,
	})
}

// signCookieValue produces "<base64(body)>.<base64(hmac(body))>" so a
// tampered body fails verification.
func signCookieValue(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func verifyCookieValue(secret []byte, signed string) ([]byte, error) {
	parts := strings.SplitN(signed, ".", 2)
	if len(parts) != 2 {
		return nil, errors.New("malformed cookie")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return nil, errors.New("cookie signature mismatch")
	}
	return body, nil
}

// ---------- helpers ----------

func randomString(n int) string {
	return base64.RawURLEncoding.EncodeToString(mustRandom(n))
}

func mustRandom(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return b
}

// sanitizeReturnTo only accepts same-origin paths so an attacker can't
// craft a /auth/login?return=https://evil.example link.
func sanitizeReturnTo(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if u.Scheme != "" || u.Host != "" {
		return ""
	}
	if !strings.HasPrefix(u.Path, "/") {
		return ""
	}
	return u.Path + (func() string {
		if u.RawQuery != "" {
			return "?" + u.RawQuery
		}
		return ""
	}())
}
