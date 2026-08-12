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
	ProtocolGRPC     Protocol = "grpc"
	ProtocolHTTP     Protocol = "http/protobuf"
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

	traceSampler    sdktrace.Sampler
	batchTimeout    time.Duration
	maxExportBatch  int
	maxQueueSize    int

	disableTraces  bool
	disableMetrics bool

	propagators []propagation.TextMapPropagator
}

func defaultConfig() *config {
	return &config{
		serviceName:    "unknown-service",
		protocol:       ProtocolGRPC,
		endpoint:       "localhost:4317",
		insecure:       true,
		traceSampler:   sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0)),
		batchTimeout:   5 * time.Second,
		maxExportBatch: 512,
		maxQueueSize:   2048,
		propagators: []propagation.TextMapPropagator{
			propagation.TraceContext{},
			propagation.Baggage{},
		},
	}
}

// WithServiceName sets service.name resource attribute. Overrides OTEL_SERVICE_NAME env.
func WithServiceName(name string) Option {
	return func(c *config) { c.serviceName = name }
}

// WithServiceVersion sets service.version resource attribute.
func WithServiceVersion(v string) Option {
	return func(c *config) { c.serviceVersion = v }
}

// WithEnvironment sets deployment.environment resource attribute (e.g. "prod", "staging").
func WithEnvironment(env string) Option {
	return func(c *config) { c.environment = env }
}

// WithResourceAttrs appends arbitrary resource attributes.
func WithResourceAttrs(kv ...attribute.KeyValue) Option {
	return func(c *config) { c.resourceAttrs = append(c.resourceAttrs, kv...) }
}

// WithProtocol selects gRPC (default) or HTTP/protobuf for OTLP transport.
func WithProtocol(p Protocol) Option {
	return func(c *config) {
		if p != "" {
			c.protocol = p
		}
	}
}

// WithEndpoint overrides the OTLP collector endpoint.
// Default: "localhost:4317" for gRPC, "http://localhost:4318" for HTTP.
// Also honors OTEL_EXPORTER_OTLP_ENDPOINT env.
func WithEndpoint(url string) Option {
	return func(c *config) { c.endpoint = url }
}

// WithHeaders adds headers to every OTLP export request (e.g. auth tokens).
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

// WithInsecure toggles TLS. Default true (assumes in-cluster collector). Set false for public collectors.
func WithInsecure(insecure bool) Option {
	return func(c *config) { c.insecure = insecure }
}

// WithTraceSampler overrides the trace sampler. Default: ParentBased(AlwaysSample) —
// tune with TraceIDRatioBased(0.01) for 1% sampling under high load.
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

// WithMaxExportBatchSize caps the number of spans per export (default 512).
func WithMaxExportBatchSize(n int) Option {
	return func(c *config) { c.maxExportBatch = n }
}

// WithMaxQueueSize caps the in-memory span queue before dropping (default 2048).
func WithMaxQueueSize(n int) Option {
	return func(c *config) { c.maxQueueSize = n }
}

// WithoutTraces disables the trace pipeline entirely.
func WithoutTraces() Option {
	return func(c *config) { c.disableTraces = true }
}

// WithoutMetrics disables the metrics pipeline entirely.
func WithoutMetrics() Option {
	return func(c *config) { c.disableMetrics = true }
}

// WithPropagators overrides the global TextMapPropagator set.
// Default: TraceContext + Baggage. Add b3, jaeger, etc. as needed.
func WithPropagators(props ...propagation.TextMapPropagator) Option {
	return func(c *config) {
		if len(props) > 0 {
			c.propagators = props
		}
	}
}
