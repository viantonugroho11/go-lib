package otel

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/noop"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Go OTel SDK does not yet install a global LoggerProvider through the top-level
// otel package (unlike Tracer/Meter). We keep one here so Logger(name) works out
// of the box and the wire-up in Init has somewhere to publish.
var globalLoggerProviderBox atomic.Value // holds log.LoggerProvider

func init() {
	globalLoggerProviderBox.Store(loggerProviderBox{p: noop.NewLoggerProvider()})
}

// loggerProviderBox exists so atomic.Value can hold different concrete impls
// (noop, sdklog) under a single struct type.
type loggerProviderBox struct{ p log.LoggerProvider }

func setGlobalLoggerProvider(p log.LoggerProvider) {
	if p == nil {
		p = noop.NewLoggerProvider()
	}
	globalLoggerProviderBox.Store(loggerProviderBox{p: p})
}

func globalLoggerProvider() log.LoggerProvider {
	if v, ok := globalLoggerProviderBox.Load().(loggerProviderBox); ok {
		return v.p
	}
	return noop.NewLoggerProvider()
}

// Logger returns a named otel logger from the global logger provider.
// Wire it into a zap/slog adapter, or use SpanContextInfo to enrich your existing
// structured logger with trace_id/span_id fields.
func Logger(name string) log.Logger { return globalLoggerProvider().Logger(name) }

func newLoggerProvider(ctx context.Context, cfg *config, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exporter, err := newLogExporter(ctx, cfg)
	if err != nil {
		return nil, err
	}
	bp := sdklog.NewBatchProcessor(exporter,
		sdklog.WithExportInterval(cfg.batchTimeout),
		sdklog.WithExportMaxBatchSize(cfg.maxExportBatch),
		sdklog.WithMaxQueueSize(cfg.maxQueueSize),
	)
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(bp),
	), nil
}

func newLogExporter(ctx context.Context, cfg *config) (sdklog.Exporter, error) {
	if cfg.stdoutExporter {
		return stdoutlog.New()
	}
	switch cfg.protocol {
	case ProtocolHTTP:
		opts := []otlploghttp.Option{otlploghttp.WithEndpointURL(cfg.endpoint)}
		if cfg.insecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		if len(cfg.headers) > 0 {
			opts = append(opts, otlploghttp.WithHeaders(cfg.headers))
		}
		if cfg.tlsCfg != nil {
			opts = append(opts, otlploghttp.WithTLSClientConfig(cfg.tlsCfg))
		}
		return otlploghttp.New(ctx, opts...)
	default:
		opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(cfg.endpoint)}
		if cfg.insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		if len(cfg.headers) > 0 {
			opts = append(opts, otlploggrpc.WithHeaders(cfg.headers))
		}
		if cfg.tlsCfg != nil {
			opts = append(opts, otlploggrpc.WithTLSCredentials(credsFromTLS(cfg.tlsCfg)))
		}
		return otlploggrpc.New(ctx, opts...)
	}
}
