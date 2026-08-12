package otel

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestLogsPipelineWiredByDefault proves Init installs a real logger provider (not noop)
// and Reset restores noop afterwards.
func TestLogsPipelineWiredByDefault(t *testing.T) {
	Reset()
	baseline := globalLoggerProvider()
	if got, want := typeName(baseline), typeName(noop.NewLoggerProvider()); got != want {
		t.Fatalf("baseline provider = %s, want %s", got, want)
	}

	sd, err := Init(context.Background(),
		WithServiceName("test"),
		WithoutTraces(),
		WithoutMetrics(),
		WithStdoutExporter(), // avoid dialing OTLP for the logs pipeline
	)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	// After init: NOT noop.
	if typeName(globalLoggerProvider()) == typeName(noop.NewLoggerProvider()) {
		t.Fatal("logger provider still noop after Init — logs pipeline not wired")
	}
	_ = sd(context.Background())
	// After shutdown: back to noop.
	if typeName(globalLoggerProvider()) != typeName(noop.NewLoggerProvider()) {
		t.Fatal("logger provider not restored to noop after shutdown")
	}
}

// TestWithoutLogs skips the logger provider install.
func TestWithoutLogs(t *testing.T) {
	Reset()
	sd, err := Init(context.Background(),
		WithServiceName("test"),
		WithoutTraces(),
		WithoutMetrics(),
		WithoutLogs(),
	)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer sd(context.Background())
	if typeName(globalLoggerProvider()) != typeName(noop.NewLoggerProvider()) {
		t.Fatal("logger provider replaced despite WithoutLogs")
	}
}

// TestStdoutExporterUsedWhenSet — Init with WithStdoutExporter must not attempt
// an OTLP dial. Proxy: Init succeeds with WithStdoutExporter even without a collector.
func TestStdoutExporterUsedWhenSet(t *testing.T) {
	Reset()
	sd, err := Init(context.Background(),
		WithServiceName("test"),
		WithStdoutExporter(),
	)
	if err != nil {
		t.Fatalf("stdout init: %v", err)
	}
	_ = sd(context.Background())
}

// TestPrometheusExporterRegistersHandler — WithPrometheusExporter attaches a /metrics
// scrape handler that returns Prometheus text format.
func TestPrometheusExporterRegistersHandler(t *testing.T) {
	Reset()
	mux := http.NewServeMux()
	sd, err := Init(context.Background(),
		WithServiceName("test"),
		WithoutTraces(),
		WithoutLogs(),
		WithStdoutExporter(), // metric OTLP exporter avoided; prom reader still attached
		WithPrometheusExporter(mux, "/metrics"),
	)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer sd(context.Background())

	// Emit one metric so the scrape has something to render.
	ctr, _ := Meter("bench").Int64Counter("test.hits")
	ctr.Add(context.Background(), 1)

	srv := httptest.NewServer(mux)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("scrape: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body := readAll(t, resp)
	if !strings.Contains(body, "test_hits") {
		t.Fatalf("Prometheus body missing metric 'test_hits' (len=%d)", len(body))
	}
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	buf := make([]byte, 0, 16384)
	tmp := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf)
}

// TestTLSConfigForcesInsecureFalse — WithTLSConfig overrides WithInsecure(true).
func TestTLSConfigForcesInsecureFalse(t *testing.T) {
	cfg := defaultConfig()
	WithInsecure(true)(cfg)
	WithTLSConfig(&tls.Config{ServerName: "collector.example"})(cfg)
	if cfg.insecure {
		t.Fatal("TLS config should disable insecure")
	}
	if cfg.tlsCfg == nil {
		t.Fatal("tlsCfg not set")
	}
}

// TestExtraSpanProcessorRuns — WithSpanProcessor appends to the trace pipeline.
// Use an in-memory recorder as the extra processor; assert it saw the span.
func TestExtraSpanProcessorRuns(t *testing.T) {
	Reset()
	rec := tracetest.NewSpanRecorder()
	sd, err := Init(context.Background(),
		WithServiceName("test"),
		WithoutMetrics(),
		WithoutLogs(),
		WithStdoutExporter(),
		WithSpanProcessor(rec),
	)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer sd(context.Background())

	_, span := Tracer("t").Start(context.Background(), "op")
	span.End()

	// Recorder is queried after End; polling not required for the sync path.
	if len(rec.Ended()) == 0 {
		t.Fatal("extra processor did not receive the span")
	}
}

// TestPropagatorHelpers — PropagatorB3 and PropagatorJaeger install known field names.
func TestPropagatorHelpers(t *testing.T) {
	if want, got := "b3", firstField(PropagatorB3()); !strings.Contains(strings.ToLower(got), want) {
		t.Errorf("B3 first field = %q, want contains %q", got, want)
	}
	if want, got := "uber-trace-id", firstField(PropagatorJaeger()); got != want {
		t.Errorf("Jaeger first field = %q, want %q", got, want)
	}
}

// TestCompositePropagatorWithB3 — combining TraceContext + B3 exposes both.
func TestCompositePropagatorWithB3(t *testing.T) {
	Reset()
	sd, err := Init(context.Background(),
		WithServiceName("test"),
		WithoutTraces(),
		WithoutMetrics(),
		WithoutLogs(),
		WithPropagators(propagation.TraceContext{}, PropagatorB3()),
	)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	defer sd(context.Background())

	fields := map[string]bool{}
	// Composite Fields() aggregates children.
	for _, f := range firstFields(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, PropagatorB3())) {
		fields[strings.ToLower(f)] = true
	}
	if !fields["traceparent"] {
		t.Error("missing traceparent")
	}
	// B3 emits b3 (single) or x-b3-* (multi) depending on config; just check something b3-ish.
	hasB3 := false
	for k := range fields {
		if strings.Contains(k, "b3") {
			hasB3 = true
			break
		}
	}
	if !hasB3 {
		t.Errorf("no b3-family field in composite; got %v", fields)
	}
}

// helpers

func firstField(p propagation.TextMapPropagator) string {
	fs := p.Fields()
	if len(fs) == 0 {
		return ""
	}
	return fs[0]
}

func firstFields(p propagation.TextMapPropagator) []string {
	return p.Fields()
}

func typeName(v any) string {
	return fmt.Sprintf("%T", v)
}
