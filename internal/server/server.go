package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/guipguia/yafu/internal/api"
	"github.com/guipguia/yafu/internal/audit"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
	"github.com/guipguia/yafu/internal/web"
)

type Config struct {
	Addr     string
	Logger   *slog.Logger
	Registry cluster.Registry

	// Auth bundles the API middleware + optional /auth/* handlers
	// (login/callback/logout for OIDC). Required — server.New panics on nil.
	Auth *auth.AuthSet

	// Policy is consulted by handlers to filter results per identity.
	// The zero value denies everything; main.go uses
	// auth.DefaultAllowAllPolicy when --rbac-file is not provided.
	Policy auth.Policy

	// Audit receives one Record per privileged action. nil → no-op
	// (but main.go always provides one writing to stdout).
	Audit *audit.Logger
}

type Server struct {
	cfg  Config
	http *http.Server
}

func New(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Auth == nil || cfg.Auth.Middleware == nil {
		panic("server.New: Config.Auth.Middleware is required")
	}

	mux := http.NewServeMux()

	// Public routes — kubelet probes, prometheus scrape, embedded UI bundle.
	mux.Handle("GET /metrics", promhttp.Handler())
	api.RegisterPublic(mux)
	web.Register(mux)

	// /auth/* — public so an unauthenticated request can reach the IdP
	// and come back. Only registered when the AuthSet provides handlers
	// (OIDC mode, today).
	if cfg.Auth.LoginHandler != nil {
		mux.HandleFunc("GET /auth/login", cfg.Auth.LoginHandler)
	}
	if cfg.Auth.CallbackHandler != nil {
		mux.HandleFunc("GET /auth/callback", cfg.Auth.CallbackHandler)
	}
	if cfg.Auth.LogoutHandler != nil {
		mux.HandleFunc("GET /auth/logout", cfg.Auth.LogoutHandler)
	}

	// Authenticated API routes mounted on a sub-mux behind the middleware.
	apiMux := http.NewServeMux()
	api.RegisterAPI(apiMux, api.Deps{
		Registry: cfg.Registry,
		Policy:   cfg.Policy,
		Audit:    cfg.Audit,
	})
	mux.Handle("/api/", cfg.Auth.Middleware(apiMux))

	handler := chain(mux,
		withRequestID(),
		withRecover(cfg.Logger),
		withObservability(cfg.Logger),
	)

	return &Server{
		cfg: cfg,
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.cfg.Logger.Info("server listening", "addr", s.cfg.Addr)
		errCh <- s.http.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		s.cfg.Logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	}
}
