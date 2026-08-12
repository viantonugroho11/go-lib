package otel

import (
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

	traceSampler   sdktrace.Sampler
	batchTimeout   time.Duration
	metricInterval time.Duration
	maxExportBatch int
	maxQueueSize   int

	disableTraces  bool
	disableMetrics bool

	propagators  []propagation.TextMapPropagator
	errorHandler ErrorHandlerFunc
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

// --- sampling + batching ---

func WithTraceSampler(s sdktrace.Sampler) Option {
	return func(c *config) {
		if s != nil {
			c.traceSampler = s
		}
	}
}

// WithBatchTimeout sets the trace batch flush timeout (default 5s).
func WithBatchTimeout(d time.Duration) Option {
	return func(c *config) { c.batchTimeout = d }
}

// WithMetricInterval sets the metric periodic-reader collection interval (default 30s).
// Split from BatchTimeout so metrics can run slower than traces (fixed in v0.1.2).
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

// --- propagators ---

func WithPropagators(props ...propagation.TextMapPropagator) Option {
	return func(c *config) {
		if len(props) > 0 {
			c.propagators = props
		}
	}
}

// --- error handling ---

// ErrorHandlerFunc receives errors reported by the OTel SDK itself
// (export failures, invalid arguments, dropped spans/metrics).
type ErrorHandlerFunc func(err error)

// Handle satisfies otel.ErrorHandler.
func (f ErrorHandlerFunc) Handle(err error) { f(err) }

// WithErrorHandler installs a callback for OTel SDK internal errors.
// Without this, errors go to the SDK's default logr sink — visible on stderr but
// disconnected from your logger. Wire this to xlog or slog for unified error surfacing.
func WithErrorHandler(fn ErrorHandlerFunc) Option {
	return func(c *config) { c.errorHandler = fn }
}
