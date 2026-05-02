package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	yafuv1alpha1 "github.com/guipguia/yafu/api/v1alpha1"
	"github.com/guipguia/yafu/internal/audit"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
	"github.com/guipguia/yafu/internal/controllers"
	"github.com/guipguia/yafu/internal/metrics"
	"github.com/guipguia/yafu/internal/server"
	"github.com/guipguia/yafu/internal/tracing"
	"github.com/guipguia/yafu/internal/version"
	"github.com/guipguia/yafu/internal/watch"
)

func main() {
	var (
		addr        = flag.String("addr", ":8080", "HTTP listen address")
		logLevel    = flag.String("log-level", "info", "log level: debug, info, warn, error")
		showVersion = flag.Bool("version", false, "print version and exit")

		clusterMode  = flag.String("cluster-mode", "auto", "cluster source: 'file', 'crd', or 'auto' (file when --config-file/--kubeconfig set, else crd)")
		configFile   = flag.String("config-file", "", "path to YAML cluster config file (file mode)")
		ownNamespace = flag.String("namespace", envOr("POD_NAMESPACE", "yafu-system"), "yafu's own namespace; used to resolve Secret refs without an explicit namespace")
		metricsAddr  = flag.String("metrics-addr", ":8081", "controller-runtime metrics listen address (CRD mode only; \"0\" disables)")
		probeAddr    = flag.String("probe-addr", ":8082", "controller-runtime healthz listen address (CRD mode only; \"0\" disables)")

		authMode = flag.String("auth-mode", "anonymous", "request authentication: 'anonymous' (dev), 'header' (trust X-Forwarded-* from a proxy), or 'oidc'.")
		rbacFile = flag.String("rbac-file", "", "path to YAML RBAC policy. When unset, every authenticated user gets full access (a WARN is logged at startup).")

		oidcIssuer           = flag.String("oidc-issuer", envOr("YAFU_OIDC_ISSUER", ""), "OIDC issuer URL (e.g. https://accounts.google.com). Required when --auth-mode=oidc.")
		oidcClientID         = flag.String("oidc-client-id", envOr("YAFU_OIDC_CLIENT_ID", ""), "OIDC client_id. Required when --auth-mode=oidc.")
		oidcClientSecretFile = flag.String("oidc-client-secret-file", envOr("YAFU_OIDC_CLIENT_SECRET_FILE", ""), "Path to a file containing the OIDC client_secret. Preferred over --oidc-client-secret for non-leaky deployments.")
		oidcClientSecret     = flag.String("oidc-client-secret", envOr("YAFU_OIDC_CLIENT_SECRET", ""), "OIDC client_secret (consider --oidc-client-secret-file instead).")
		oidcRedirectURL      = flag.String("oidc-redirect-url", envOr("YAFU_OIDC_REDIRECT_URL", ""), "Public URL of yafu's /auth/callback (e.g. https://yafu.example.com/auth/callback). Required when --auth-mode=oidc.")
		oidcScopes           = flag.String("oidc-scopes", envOr("YAFU_OIDC_SCOPES", "openid,email,profile,groups"), "Comma-separated OAuth2 scopes.")
		oidcGroupsClaim      = flag.String("oidc-groups-claim", envOr("YAFU_OIDC_GROUPS_CLAIM", "groups"), "ID-token claim that holds group names; varies by IdP.")
		oidcCookieSecretFile = flag.String("oidc-cookie-secret-file", envOr("YAFU_OIDC_COOKIE_SECRET_FILE", ""), "Path to a file containing the cookie-signing secret. If unset, a random secret is generated at startup.")
		oidcSecureCookie     = flag.Bool("oidc-secure-cookie", envOr("YAFU_OIDC_SECURE_COOKIE", "true") != "false", "Set Secure flag on cookies. Disable for HTTP dev only.")

		otelEndpoint = flag.String("otel-endpoint", envOr("OTEL_EXPORTER_OTLP_ENDPOINT", ""), "OTLP HTTP collector endpoint (e.g. http://otel-collector:4318). Empty disables tracing.")
		otelInsecure = flag.Bool("otel-insecure", envOr("OTEL_EXPORTER_OTLP_INSECURE", "true") != "false", "Skip TLS for the OTLP exporter (cluster-internal collectors typically use HTTP).")
		otelSample   = flag.Float64("otel-sample-rate", parseFloat(envOr("OTEL_TRACES_SAMPLER_ARG", "1.0"), 1.0), "Head-based trace sampler ratio in [0.0, 1.0].")
	)
	// --kubeconfig is registered by sigs.k8s.io/controller-runtime/pkg/client/config
	// via package init(); we read its value after parsing.
	flag.Parse()
	kubeconfig := lookupFlag("kubeconfig")

	if *showVersion {
		fmt.Println(version.String())
		return
	}

	logger := newLogger(*logLevel)
	slog.SetDefault(logger)
	crlog.SetLogger(zap.New(zap.UseDevMode(true), zap.Level(zapcore.InfoLevel)))

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	traceShutdown, err := tracing.Setup(ctx, tracing.Config{
		Endpoint:       *otelEndpoint,
		Insecure:       *otelInsecure,
		SampleRate:     *otelSample,
		ServiceVersion: version.Version,
	}, logger)
	if err != nil {
		logger.Error("tracing init", "err", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		if err := traceShutdown(shutdownCtx); err != nil {
			logger.Warn("trace exporter shutdown", "err", err)
		}
	}()

	mode := pickMode(*clusterMode, *configFile, kubeconfig)
	logger.Info("cluster mode", "mode", mode)

	registry, runManager, err := buildRegistry(ctx, logger, mode, *configFile, kubeconfig, *ownNamespace, *metricsAddr, *probeAddr)
	if err != nil {
		logger.Error("registry init", "err", err)
		os.Exit(1)
	}

	if registry != nil {
		metrics.MustRegisterRegistry(cluster.MetricsSnapshot(registry))
	}

	if runManager != nil {
		go func() {
			if err := runManager(ctx); err != nil {
				logger.Error("controller manager stopped", "err", err)
				cancel()
			}
		}()
	}

	parsedAuthMode, err := auth.ParseMode(*authMode)
	if err != nil {
		logger.Error("auth mode", "err", err)
		os.Exit(1)
	}
	var authSet *auth.AuthSet
	switch parsedAuthMode {
	case auth.ModeOIDC:
		oidcCfg, err := buildOIDCConfig(
			*oidcIssuer, *oidcClientID, *oidcClientSecret, *oidcClientSecretFile,
			*oidcRedirectURL, *oidcScopes, *oidcGroupsClaim,
			*oidcCookieSecretFile, *oidcSecureCookie,
		)
		if err != nil {
			logger.Error("oidc config", "err", err)
			os.Exit(1)
		}
		authSet, err = auth.NewOIDC(ctx, oidcCfg)
		if err != nil {
			logger.Error("oidc init", "err", err)
			os.Exit(1)
		}
	default:
		authSet, err = auth.NewSet(parsedAuthMode)
		if err != nil {
			logger.Error("auth init", "err", err)
			os.Exit(1)
		}
	}
	logger.Info("auth mode", "mode", parsedAuthMode)
	if parsedAuthMode == auth.ModeAnonymous {
		logger.Warn("auth mode 'anonymous' allows unauthenticated access — do not use in production")
	}

	policy := auth.DefaultAllowAllPolicy
	if *rbacFile != "" {
		p, err := auth.LoadPolicyFile(*rbacFile)
		if err != nil {
			logger.Error("rbac policy", "err", err)
			os.Exit(1)
		}
		policy = p
		logger.Info("rbac policy", "file", *rbacFile, "rules", len(policy.Rules), "default", string(policy.DefaultAction))
	} else if parsedAuthMode != auth.ModeAnonymous {
		logger.Warn("no --rbac-file set; every authenticated user has full access")
	}

	auditLog := audit.New(os.Stdout)

	hub := watch.NewHub()
	if registry != nil {
		go (&watch.Manager{
			Hub:      hub,
			Registry: registry,
			Logger:   logger,
		}).Run(ctx)
	}

	srv := server.New(server.Config{
		Addr:     *addr,
		Logger:   logger,
		Registry: registry,
		Auth:     authSet,
		Policy:   policy,
		Audit:    auditLog,
		Hub:      hub,
	})

	if err := srv.Run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

// pickMode resolves "auto" by preferring file mode whenever yafu can
// reach a kubeconfig — explicit flag, KUBECONFIG env, or ~/.kube/config.
// Otherwise (running in-cluster) it falls back to CRD mode.
func pickMode(mode, configFile, kubeconfig string) string {
	if mode != "auto" {
		return mode
	}
	if configFile != "" || kubeconfig != "" {
		return "file"
	}
	if os.Getenv("KUBECONFIG") != "" || homeKubeconfigExists() {
		return "file"
	}
	return "crd"
}

// buildRegistry constructs the registry for the chosen mode and returns
// (when applicable) a function that runs the controller-runtime manager.
func buildRegistry(
	ctx context.Context,
	logger *slog.Logger,
	mode, configFile, kubeconfig, ownNamespace, metricsAddr, probeAddr string,
) (cluster.Registry, func(context.Context) error, error) {
	switch mode {
	case "file":
		fileCfg := &cluster.FileConfig{}
		if configFile != "" {
			loaded, err := cluster.LoadFileConfig(configFile)
			if err != nil {
				return nil, nil, err
			}
			fileCfg = loaded
		} else if kubeconfig != "" || os.Getenv("KUBECONFIG") != "" || homeKubeconfigExists() {
			fileCfg.DiscoverContexts = &cluster.DiscoverConfig{Kubeconfig: kubeconfig}
		}
		fr, err := cluster.NewFileRegistry(fileCfg, logger)
		if err != nil {
			return nil, nil, err
		}
		go fr.Start(ctx)
		return fr, nil, nil

	case "crd":
		homeCfg, err := config.GetConfig()
		if err != nil {
			return nil, nil, fmt.Errorf("get home cluster config: %w", err)
		}
		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(yafuv1alpha1.AddToScheme(scheme))

		mgr, err := ctrl.NewManager(homeCfg, ctrl.Options{
			Scheme:                 scheme,
			Metrics:                metricsserver.Options{BindAddress: metricsAddr},
			HealthProbeBindAddress: probeAddr,
			LeaderElection:         false,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("new manager: %w", err)
		}

		crdReg := cluster.NewCRDRegistry()
		if err := (&controllers.ClusterReconciler{
			Client:       mgr.GetClient(),
			Registry:     crdReg,
			HomeConfig:   homeCfg,
			OwnNamespace: ownNamespace,
		}).SetupWithManager(mgr); err != nil {
			return nil, nil, fmt.Errorf("setup controller: %w", err)
		}

		return crdReg, mgr.Start, nil

	default:
		return nil, nil, fmt.Errorf("unknown cluster-mode %q (want file|crd|auto)", mode)
	}
}

func homeKubeconfigExists() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	_, err = os.Stat(home + "/.kube/config")
	return err == nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func lookupFlag(name string) string {
	if f := flag.Lookup(name); f != nil {
		return f.Value.String()
	}
	return ""
}

func buildOIDCConfig(
	issuer, clientID, clientSecret, clientSecretFile,
	redirectURL, scopesCSV, groupsClaim, cookieSecretFile string,
	secureCookie bool,
) (auth.OIDCConfig, error) {
	if issuer == "" || clientID == "" || redirectURL == "" {
		return auth.OIDCConfig{}, fmt.Errorf("--oidc-issuer, --oidc-client-id and --oidc-redirect-url are required for auth-mode=oidc")
	}

	if clientSecretFile != "" {
		b, err := os.ReadFile(clientSecretFile)
		if err != nil {
			return auth.OIDCConfig{}, fmt.Errorf("read client_secret file %s: %w", clientSecretFile, err)
		}
		clientSecret = strings.TrimSpace(string(b))
	}

	var cookieSecret []byte
	if cookieSecretFile != "" {
		b, err := os.ReadFile(cookieSecretFile)
		if err != nil {
			return auth.OIDCConfig{}, fmt.Errorf("read cookie_secret file %s: %w", cookieSecretFile, err)
		}
		cookieSecret = []byte(strings.TrimSpace(string(b)))
	}

	scopes := []string{}
	for _, s := range strings.Split(scopesCSV, ",") {
		if t := strings.TrimSpace(s); t != "" {
			scopes = append(scopes, t)
		}
	}

	return auth.OIDCConfig{
		Issuer:       issuer,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		GroupsClaim:  groupsClaim,
		CookieSecret: cookieSecret,
		SecureCookie: secureCookie,
	}, nil
}

// parseFloat is a small helper that returns def when v can't be
// parsed. Used for the OTEL_TRACES_SAMPLER_ARG env fallback so a
// malformed env value doesn't fail startup — yafu just runs with
// the default sample rate.
func parseFloat(v string, def float64) float64 {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
