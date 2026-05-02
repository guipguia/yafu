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
	"syscall"

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
	"github.com/guipguia/yafu/internal/cluster"
	"github.com/guipguia/yafu/internal/controllers"
	"github.com/guipguia/yafu/internal/server"
	"github.com/guipguia/yafu/internal/version"
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

	mode := pickMode(*clusterMode, *configFile, kubeconfig)
	logger.Info("cluster mode", "mode", mode)

	registry, runManager, err := buildRegistry(ctx, logger, mode, *configFile, kubeconfig, *ownNamespace, *metricsAddr, *probeAddr)
	if err != nil {
		logger.Error("registry init", "err", err)
		os.Exit(1)
	}

	if runManager != nil {
		go func() {
			if err := runManager(ctx); err != nil {
				logger.Error("controller manager stopped", "err", err)
				cancel()
			}
		}()
	}

	srv := server.New(server.Config{
		Addr:     *addr,
		Logger:   logger,
		Registry: registry,
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
