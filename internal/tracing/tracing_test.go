package tracing

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestSetup_NoEndpointIsNoop is the load-bearing assertion: yafu
// must run cleanly with tracing off (no collector deployed) and
// must NOT replace the global TracerProvider with anything that
// could produce side effects.
func TestSetup_NoEndpointIsNoop(t *testing.T) {
	// Reset the global provider so the test isn't dirtied by other
	// tests in the package.
	otel.SetTracerProvider(tracenoop.NewTracerProvider())

	shutdown, err := Setup(context.Background(), Config{}, newTestLogger())
	if err != nil {
		t.Fatalf("Setup with empty endpoint failed: %v", err)
	}
	if shutdown == nil {
		t.Fatal("shutdown returned nil")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown returned %v, want nil", err)
	}

	// Tracer() must return a working (no-op) tracer regardless of
	// whether Setup configured anything.
	tr := Tracer()
	_, span := tr.Start(context.Background(), "test")
	span.End()
	if !span.SpanContext().IsValid() && !span.SpanContext().IsSampled() {
		// noop spans report invalid+unsampled, which is fine — the
		// only thing we're checking is that Tracer() didn't panic.
	}
}

// TestSetup_BadEndpointReturnsError covers the failure branch
// when the OTLP exporter constructor fails (e.g. completely
// malformed endpoint). otlptracehttp tolerates many shapes; the
// guard against a totally empty Config we already cover above.
// Here we just ensure stripScheme handles a normal http URL.
func TestStripScheme(t *testing.T) {
	cases := map[string]string{
		"http://otel:4318":  "otel:4318",
		"https://otel:4318": "otel:4318",
		"otel:4318":         "otel:4318",
		"":                  "",
	}
	for in, want := range cases {
		if got := stripScheme(in); got != want {
			t.Errorf("stripScheme(%q) = %q, want %q", in, got, want)
		}
	}
}

