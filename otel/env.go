package otel

import (
	"context"
	"os"
	"strconv"
	"strings"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitFromEnv reads standard OTEL_* env vars and calls Init.
//
// Recognized vars (in addition to those the SDK itself reads via resource.WithFromEnv):
//   - GO_LIB_OTEL_ENDPOINT           — overrides OTEL_EXPORTER_OTLP_ENDPOINT
//   - GO_LIB_OTEL_PROTOCOL           — "grpc" (default) or "http/protobuf"
//   - GO_LIB_OTEL_INSECURE           — "true" (default) or "false"
//   - GO_LIB_OTEL_SERVICE_NAME       — overrides OTEL_SERVICE_NAME
//   - GO_LIB_OTEL_SERVICE_VERSION
//   - GO_LIB_OTEL_ENVIRONMENT        — deployment.environment
//   - GO_LIB_OTEL_TRACE_SAMPLE_RATIO — float in [0, 1]; 1.0 = always, 0.0 = never
//   - GO_LIB_OTEL_DISABLE_TRACES     — "true" to skip trace pipeline
//   - GO_LIB_OTEL_DISABLE_METRICS    — "true" to skip metric pipeline
//
// Extra opts override env values.
func InitFromEnv(ctx context.Context, extra ...Option) (ShutdownFunc, error) {
	opts := envOptions()
	opts = append(opts, extra...)
	return Init(ctx, opts...)
}

func envOptions() []Option {
	var opts []Option
	if v := firstEnv("GO_LIB_OTEL_SERVICE_NAME", "OTEL_SERVICE_NAME"); v != "" {
		opts = append(opts, WithServiceName(v))
	}
	if v := os.Getenv("GO_LIB_OTEL_SERVICE_VERSION"); v != "" {
		opts = append(opts, WithServiceVersion(v))
	}
	if v := os.Getenv("GO_LIB_OTEL_ENVIRONMENT"); v != "" {
		opts = append(opts, WithEnvironment(v))
	}
	if v := firstEnv("GO_LIB_OTEL_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		opts = append(opts, WithEndpoint(v))
	}
	if v := strings.ToLower(firstEnv("GO_LIB_OTEL_PROTOCOL", "OTEL_EXPORTER_OTLP_PROTOCOL")); v != "" {
		switch v {
		case "http", "http/protobuf":
			opts = append(opts, WithProtocol(ProtocolHTTP))
		case "grpc":
			opts = append(opts, WithProtocol(ProtocolGRPC))
		}
	}
	if v := os.Getenv("GO_LIB_OTEL_INSECURE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			opts = append(opts, WithInsecure(b))
		}
	}
	if v := os.Getenv("GO_LIB_OTEL_TRACE_SAMPLE_RATIO"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			opts = append(opts, WithTraceSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(f))))
		}
	}
	if isTrue(os.Getenv("GO_LIB_OTEL_DISABLE_TRACES")) {
		opts = append(opts, WithoutTraces())
	}
	if isTrue(os.Getenv("GO_LIB_OTEL_DISABLE_METRICS")) {
		opts = append(opts, WithoutMetrics())
	}
	return opts
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func isTrue(s string) bool {
	b, err := strconv.ParseBool(s)
	return err == nil && b
}
