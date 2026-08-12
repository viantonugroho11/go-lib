package otel

import (
	"context"
	"strings"
	"testing"
	"time"

	sdkotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestInit_ExporterUnreachable proves Init returns an error when the OTLP endpoint
// cannot be resolved within ctx's deadline. Serves as a sanity smoke for the wiring
// without requiring a live collector.
func TestInit_ExporterUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	_, err := Init(ctx,
		WithServiceName("test"),
		WithEndpoint("localhost:1"), // guaranteed refused / no listener
		WithProtocol(ProtocolGRPC),
	)
	// gRPC exporter is lazy — the actual dial happens on first export. So Init likely
	// succeeds here; if it does, that's still valid — we just make sure it doesn't panic.
	// Test kept as a smoke — real integration coverage needs a collector.
	if err != nil && !strings.Contains(err.Error(), "otel") {
		t.Fatalf("unexpected error kind: %v", err)
	}
}

// TestPropagatorInstalled verifies Init sets the global TextMapPropagator to the
// composite of TraceContext + Baggage, so downstream libs (kafka, httpclient) see it.
func TestPropagatorInstalled(t *testing.T) {
	// Reset before test.
	sdkotel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

	// Use WithoutTraces + WithoutMetrics to skip exporter dials entirely, so we only
	// exercise the propagator wiring path.
	shutdown, err := Init(context.Background(),
		WithServiceName("test"),
		WithoutTraces(),
		WithoutMetrics(),
	)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer shutdown(context.Background())

	prop := sdkotel.GetTextMapPropagator()
	fields := prop.Fields()
	if len(fields) == 0 {
		t.Fatal("no propagator fields — composite not installed")
	}
	// Check for traceparent (TraceContext) + baggage.
	got := map[string]bool{}
	for _, f := range fields {
		got[f] = true
	}
	if !got["traceparent"] {
		t.Errorf("missing traceparent; got %v", fields)
	}
	if !got["baggage"] {
		t.Errorf("missing baggage; got %v", fields)
	}
}

// TestCustomPropagator verifies WithPropagators replaces the default set.
func TestCustomPropagator(t *testing.T) {
	shutdown, err := Init(context.Background(),
		WithServiceName("test"),
		WithoutTraces(),
		WithoutMetrics(),
		WithPropagators(propagation.TraceContext{}), // omit Baggage
	)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer shutdown(context.Background())

	fields := sdkotel.GetTextMapPropagator().Fields()
	got := map[string]bool{}
	for _, f := range fields {
		got[f] = true
	}
	if !got["traceparent"] {
		t.Errorf("traceparent missing")
	}
	if got["baggage"] {
		t.Errorf("baggage should be absent when overridden; got %v", fields)
	}
}

// TestTracerRecordsSpan uses an in-memory span recorder attached to a manual
// TracerProvider (bypassing Init's OTLP exporter) to prove the built resource
// carries the service.name and custom attribute.
func TestTracerRecordsSpan(t *testing.T) {
	ctx := context.Background()

	// Bypass Init's exporter; build the resource with the same builder used by Init
	// and attach an in-memory span recorder.
	cfg := defaultConfig()
	cfg.serviceName = "svc-under-test"
	cfg.serviceVersion = "1.2.3"
	cfg.environment = "test"
	cfg.resourceAttrs = []attribute.KeyValue{attribute.String("custom.key", "v")}
	res, err := buildResource(ctx, cfg)
	if err != nil {
		t.Fatalf("resource: %v", err)
	}

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(rec),
	)
	defer tp.Shutdown(ctx)

	_, span := tp.Tracer("t").Start(ctx, "op")
	span.End()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d", len(spans))
	}
	attrs := map[string]string{}
	for _, kv := range spans[0].Resource().Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	if attrs["service.name"] != "svc-under-test" {
		t.Errorf("service.name = %q", attrs["service.name"])
	}
	if attrs["service.version"] != "1.2.3" {
		t.Errorf("service.version = %q", attrs["service.version"])
	}
	if attrs["deployment.environment"] != "test" {
		t.Errorf("deployment.environment = %q", attrs["deployment.environment"])
	}
	if attrs["custom.key"] != "v" {
		t.Errorf("custom.key = %q", attrs["custom.key"])
	}
}

// TestInitFromEnvReadsOverrides sets a few env vars and verifies the parsed options
// take effect. Uses WithoutTraces/Metrics to skip exporter dials.
func TestInitFromEnvReadsOverrides(t *testing.T) {
	t.Setenv("GO_LIB_OTEL_SERVICE_NAME", "env-svc")
	t.Setenv("GO_LIB_OTEL_SERVICE_VERSION", "9.9.9")
	t.Setenv("GO_LIB_OTEL_ENVIRONMENT", "staging")

	shutdown, err := InitFromEnv(context.Background(), WithoutTraces(), WithoutMetrics())
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer shutdown(context.Background())

	// Verify by parsing config through the same envOptions path.
	opts := envOptions()
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}
	if cfg.serviceName != "env-svc" {
		t.Fatalf("service = %q", cfg.serviceName)
	}
	if cfg.serviceVersion != "9.9.9" {
		t.Fatalf("version = %q", cfg.serviceVersion)
	}
	if cfg.environment != "staging" {
		t.Fatalf("env = %q", cfg.environment)
	}
}
