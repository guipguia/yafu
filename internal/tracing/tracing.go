// Package tracing wires OpenTelemetry distributed tracing for yafu.
//
// Default behaviour is no-op: when no OTLP endpoint is configured
// the tracer provider is the global no-op and OTel calls compile to
// near-zero-cost stubs. Production deployments set
// OTEL_EXPORTER_OTLP_ENDPOINT (or pass --otel-endpoint) to ship
// spans to their backend of choice — Tempo, Honeycomb, Datadog,
// Grafana Cloud, etc, anything that speaks OTLP HTTP.
package tracing

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

// Config carries the tracing init knobs. Endpoint empty disables
// the exporter entirely, which is the default — yafu must run
// fine on a cluster without an OTel collector.
type Config struct {
	// Endpoint is the OTLP HTTP collector base URL, e.g.
	// "http://otel-collector.observability:4318". When empty,
	// tracing is a no-op.
	Endpoint string
	// Insecure permits HTTP (no TLS) when true. Cluster-internal
	// collectors typically run plain HTTP.
	Insecure bool
	// SampleRate is the head-based sampler ratio in [0.0, 1.0].
	// Default 1.0 — yafu is low-volume so always-on tracing is
	// affordable.
	SampleRate float64
	// ServiceVersion stamps every span with version.Version so
	// you can correlate traces with the build that emitted them.
	ServiceVersion string
}

// Setup initialises a TracerProvider per cfg and registers it as
// the OTel global. Returns a shutdown function the caller must
// invoke (defer is fine) — flushing buffered spans on exit
// matters for a short-lived process or a test.
//
// When cfg.Endpoint is empty, returns a no-op shutdown function and
// leaves the OTel global as the default no-op provider.
func Setup(ctx context.Context, cfg Config, logger *slog.Logger) (func(context.Context) error, error) {
	if cfg.Endpoint == "" {
		logger.Info("tracing disabled (no OTLP endpoint configured)")
		// Install the W3C propagator anyway so incoming traceparent
		// headers from upstream services are honoured even when we
		// don't export — the spans we'd emit are dropped, but the
		// trace context flows through downstream calls.
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		return noopShutdown, nil
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(stripScheme(cfg.Endpoint)),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exp, err := otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("create OTLP exporter: %w", err)
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("yafu"),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	rate := cfg.SampleRate
	if rate <= 0 {
		rate = 1.0
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(rate))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Info("tracing enabled",
		"endpoint", cfg.Endpoint,
		"sample_rate", rate,
		"service_version", cfg.ServiceVersion,
	)

	return tp.Shutdown, nil
}

// Tracer returns yafu's tracer. Callers use it to start spans —
// the underlying TracerProvider may be no-op or fully configured;
// either way the call is correct.
func Tracer() trace.Tracer {
	return otel.Tracer("github.com/guipguia/yafu")
}

// stripScheme is needed because otlptracehttp.WithEndpoint expects
// host[:port] without the scheme — a common configuration mistake
// is to pass the full URL.
func stripScheme(url string) string {
	for _, prefix := range []string{"https://", "http://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			return url[len(prefix):]
		}
	}
	return url
}

func noopShutdown(context.Context) error { return nil }
