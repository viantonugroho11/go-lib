package otel

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func BenchmarkTracer_StartEnd_NoOp(b *testing.B) {
	// NeverSample: spans are created but not recorded — measures the always-on overhead.
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
	defer tp.Shutdown(context.Background())
	tr := tp.Tracer("bench")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tr.Start(ctx, "op")
		span.End()
	}
}

func BenchmarkTracer_StartEnd_Sampled(b *testing.B) {
	// AlwaysSample + in-memory recorder = full span build + processor path.
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(rec),
	)
	defer tp.Shutdown(context.Background())
	tr := tp.Tracer("bench")
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tr.Start(ctx, "op")
		span.End()
	}
}

func BenchmarkBuildResource(b *testing.B) {
	cfg := defaultConfig()
	cfg.serviceName = "svc"
	cfg.serviceVersion = "1.2.3"
	cfg.environment = "prod"
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = buildResource(ctx, cfg)
	}
}
