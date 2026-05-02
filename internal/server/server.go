package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/guipguia/yafu/internal/api"
	"github.com/guipguia/yafu/internal/cluster"
	"github.com/guipguia/yafu/internal/web"
)

type Config struct {
	Addr     string
	Logger   *slog.Logger
	Registry cluster.Registry
}

type Server struct {
	cfg  Config
	http *http.Server
}

func New(cfg Config) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	api.Register(mux, api.Deps{Registry: cfg.Registry})
	web.Register(mux)

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
