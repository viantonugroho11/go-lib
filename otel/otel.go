// Package otel bootstraps OpenTelemetry for a service: resource, tracer provider,
// meter provider, and global text-map propagator. Init returns a shutdown func that
// flushes pending spans/metrics on exit; always call it.
//
// Usage:
//
//	shutdown, err := otel.Init(context.Background(),
//	    otel.WithServiceName("payments"),
//	    otel.WithServiceVersion("1.4.2"),
//	    otel.WithEnvironment("prod"),
//	    otel.WithEndpoint("otel-collector.observability:4317"),
//	    otel.WithErrorHandler(func(err error) { log.Printf("otel: %v", err) }),
//	)
//	if err != nil { log.Fatal(err) }
//	defer shutdown(context.Background())
//
//	tracer := otel.Tracer("payments/service")
//	ctx, span := tracer.Start(ctx, "ChargeCard")
//	defer span.End()
package otel

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// ShutdownFunc flushes and stops all providers. Safe to call multiple times.
type ShutdownFunc func(ctx context.Context) error

var (
	initMu      sync.Mutex
	initialized bool
)

// Init wires up global providers and the propagator. Returns a shutdown func that flushes
// pending telemetry within ctx's deadline; always call it before exit.
//
// Calling Init more than once returns an error — providers are boot-once state.
// Use Reset() in tests to re-init.
func Init(ctx context.Context, opts ...Option) (ShutdownFunc, error) {
	initMu.Lock()
	defer initMu.Unlock()
	if initialized {
		return nil, errors.New("otel: Init already called; use Reset in tests to re-init")
	}

	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	if cfg.errorHandler != nil {
		otel.SetErrorHandler(cfg.errorHandler)
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("otel: build resource: %w", err)
	}

	var shutdownFns []func(context.Context) error

	if !cfg.disableTraces {
		tp, err := newTracerProvider(ctx, cfg, res)
		if err != nil {
			return nil, fmt.Errorf("otel: tracer provider: %w", err)
		}
		otel.SetTracerProvider(tp)
		shutdownFns = append(shutdownFns, tp.Shutdown)
	}

	if !cfg.disableMetrics {
		mp, err := newMeterProvider(ctx, cfg, res)
		if err != nil {
			return nil, fmt.Errorf("otel: meter provider: %w", err)
		}
		otel.SetMeterProvider(mp)
		shutdownFns = append(shutdownFns, mp.Shutdown)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(cfg.propagators...))

	initialized = true
	shutdown := func(sctx context.Context) error {
		var errs []error
		for _, fn := range shutdownFns {
			if err := fn(sctx); err != nil {
				errs = append(errs, err)
			}
		}
		initMu.Lock()
		initialized = false
		initMu.Unlock()
		return errors.Join(errs...)
	}
	return shutdown, nil
}

// Reset clears the initialized flag so tests can call Init again. Not for production use.
func Reset() {
	initMu.Lock()
	initialized = false
	initMu.Unlock()
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer { return otel.Tracer(name) }

// Meter returns a named meter from the global provider.
func Meter(name string) metric.Meter { return otel.Meter(name) }

// SpanContextInfo pulls trace ID, span ID, and sampled flag from ctx for log correlation.
// ok is false when ctx carries no valid span.
//
// Use it to populate fields on any structured logger (xlog, slog, zap) without importing
// otel from that logger's package:
//
//	if info, ok := otel.SpanContextInfo(ctx); ok {
//	    logger.Info("event",
//	        "trace_id", info.TraceID,
//	        "span_id", info.SpanID,
//	    )
//	}
func SpanContextInfo(ctx context.Context) (SpanInfo, bool) {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return SpanInfo{}, false
	}
	return SpanInfo{
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
		Sampled: sc.IsSampled(),
	}, true
}

// SpanInfo is the compact view of a SpanContext for log correlation.
type SpanInfo struct {
	TraceID string
	SpanID  string
	Sampled bool
}

// --- internals ---

func buildResource(ctx context.Context, cfg *config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.serviceName),
	}
	if cfg.serviceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.serviceVersion))
	}
	if cfg.environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment", cfg.environment))
	}
	attrs = append(attrs, cfg.resourceAttrs...)

	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attrs...),
	)
}

func newTracerProvider(ctx context.Context, cfg *config, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := newTraceExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	bsp := sdktrace.NewBatchSpanProcessor(exporter,
		sdktrace.WithBatchTimeout(cfg.batchTimeout),
		sdktrace.WithMaxExportBatchSize(cfg.maxExportBatch),
		sdktrace.WithMaxQueueSize(cfg.maxQueueSize),
	)
	return sdktrace.NewTracerProvider(
		sdktrace.WithSampler(cfg.traceSampler),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	), nil
}

func newTraceExporter(ctx context.Context, cfg *config) (sdktrace.SpanExporter, error) {
	switch cfg.protocol {
	case ProtocolHTTP:
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(cfg.endpoint)}
		if cfg.insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		if len(cfg.headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(cfg.headers))
		}
		return otlptrace.New(ctx, otlptracehttp.NewClient(opts...))
	default:
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.endpoint)}
		if cfg.insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		if len(cfg.headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.headers))
		}
		return otlptracegrpc.New(ctx, opts...)
	}
}

func newMeterProvider(ctx context.Context, cfg *config, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := newMetricExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.metricInterval))
	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	), nil
}

func newMetricExporter(ctx context.Context, cfg *config) (sdkmetric.Exporter, error) {
	switch cfg.protocol {
	case ProtocolHTTP:
		opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpointURL(cfg.endpoint)}
		if cfg.insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		if len(cfg.headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(cfg.headers))
		}
		return otlpmetrichttp.New(ctx, opts...)
	default:
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.endpoint)}
		if cfg.insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		if len(cfg.headers) > 0 {
			opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.headers))
		}
		return otlpmetricgrpc.New(ctx, opts...)
	}
}
