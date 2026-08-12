package otel

import (
	"crypto/tls"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Protocol selects the OTLP wire format.
type Protocol string

const (
	ProtocolGRPC Protocol = "grpc"
	ProtocolHTTP Protocol = "http/protobuf"
)

// Option configures Init.
type Option func(*config)

type config struct {
	serviceName    string
	serviceVersion string
	environment    string
	resourceAttrs  []attribute.KeyValue

	protocol Protocol
	endpoint string
	headers  map[string]string
	insecure bool
	tlsCfg   *tls.Config

	traceSampler   sdktrace.Sampler
	batchTimeout   time.Duration
	metricInterval time.Duration
	maxExportBatch int
	maxQueueSize   int

	disableTraces  bool
	disableMetrics bool
	disableLogs    bool

	propagators     []propagation.TextMapPropagator
	extraProcessors []sdktrace.SpanProcessor

	promMux        *http.ServeMux
	promPath       string
	runtimeMetrics bool
	stdoutExporter bool
	errorHandler   ErrorHandlerFunc
}

func defaultConfig() *config {
	return &config{
		serviceName:    "unknown-service",
		protocol:       ProtocolGRPC,
		endpoint:       "localhost:4317",
		insecure:       true,
		traceSampler:   sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0)),
		batchTimeout:   5 * time.Second,
		metricInterval: 30 * time.Second,
		maxExportBatch: 512,
		maxQueueSize:   2048,
		propagators: []propagation.TextMapPropagator{
			propagation.TraceContext{},
			propagation.Baggage{},
		},
	}
}

// --- identity ---

func WithServiceName(name string) Option {
	return func(c *config) { c.serviceName = name }
}
func WithServiceVersion(v string) Option {
	return func(c *config) { c.serviceVersion = v }
}
func WithEnvironment(env string) Option {
	return func(c *config) { c.environment = env }
}
func WithResourceAttrs(kv ...attribute.KeyValue) Option {
	return func(c *config) { c.resourceAttrs = append(c.resourceAttrs, kv...) }
}

// --- transport ---

func WithProtocol(p Protocol) Option {
	return func(c *config) {
		if p != "" {
			c.protocol = p
		}
	}
}
func WithEndpoint(url string) Option {
	return func(c *config) { c.endpoint = url }
}
func WithHeaders(h map[string]string) Option {
	return func(c *config) {
		if c.headers == nil {
			c.headers = make(map[string]string, len(h))
		}
		for k, v := range h {
			c.headers[k] = v
		}
	}
}
func WithInsecure(insecure bool) Option {
	return func(c *config) { c.insecure = insecure }
}

// WithTLSConfig supplies a *tls.Config for OTLP transport (CA cert, mTLS, etc.).
// Wins over WithInsecure.
func WithTLSConfig(cfg *tls.Config) Option {
	return func(c *config) {
		if cfg != nil {
			c.tlsCfg = cfg
			c.insecure = false
		}
	}
}

// --- sampling + batching ---

func WithTraceSampler(s sdktrace.Sampler) Option {
	return func(c *config) {
		if s != nil {
			c.traceSampler = s
		}
	}
}
func WithBatchTimeout(d time.Duration) Option {
	return func(c *config) { c.batchTimeout = d }
}
func WithMetricInterval(d time.Duration) Option {
	return func(c *config) { c.metricInterval = d }
}
func WithMaxExportBatchSize(n int) Option {
	return func(c *config) { c.maxExportBatch = n }
}
func WithMaxQueueSize(n int) Option {
	return func(c *config) { c.maxQueueSize = n }
}

// --- opt-out ---

func WithoutTraces() Option {
	return func(c *config) { c.disableTraces = true }
}
func WithoutMetrics() Option {
	return func(c *config) { c.disableMetrics = true }
}

// WithoutLogs disables the logs pipeline entirely.
// Logs are wired by default when Init runs.
func WithoutLogs() Option {
	return func(c *config) { c.disableLogs = true }
}

// --- propagators + processors ---

func WithPropagators(props ...propagation.TextMapPropagator) Option {
	return func(c *config) {
		if len(props) > 0 {
			c.propagators = props
		}
	}
}

// WithSpanProcessor appends an extra SpanProcessor to the trace pipeline
// (e.g. debug printer, PII redactor, custom tagger). Runs alongside the batch processor.
func WithSpanProcessor(sp sdktrace.SpanProcessor) Option {
	return func(c *config) {
		if sp != nil {
			c.extraProcessors = append(c.extraProcessors, sp)
		}
	}
}

// --- metrics variants ---

// WithPrometheusExporter registers a Prometheus scrape endpoint on the given mux at path.
// Runs alongside the OTLP metric pipeline; both receive the same measurements.
// path defaults to "/metrics" when empty.
func WithPrometheusExporter(mux *http.ServeMux, path string) Option {
	return func(c *config) {
		if mux == nil {
			return
		}
		if path == "" {
			path = "/metrics"
		}
		c.promMux = mux
		c.promPath = path
	}
}

// WithRuntimeMetrics enables Go runtime auto-instrumentation:
// goroutines, GC, heap, memory stats via contrib/instrumentation/runtime.
func WithRuntimeMetrics() Option {
	return func(c *config) { c.runtimeMetrics = true }
}

// --- dev + error handling ---

// WithStdoutExporter routes traces + metrics + logs to stdout instead of OTLP.
// Wins over the OTLP exporter. Use in tests + local dev without a collector.
func WithStdoutExporter() Option {
	return func(c *config) { c.stdoutExporter = true }
}

type ErrorHandlerFunc func(err error)

func (f ErrorHandlerFunc) Handle(err error) { f(err) }

func WithErrorHandler(fn ErrorHandlerFunc) Option {
	return func(c *config) { c.errorHandler = fn }
}
