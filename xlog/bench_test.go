package xlog

import (
	"context"
	"io"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func buildBenchLogger(b *testing.B) *zap.Logger {
	b.Helper()
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(io.Discard), zapcore.InfoLevel)
	l := zap.New(core)
	undo := zap.ReplaceGlobals(l)
	b.Cleanup(undo)
	return l
}

func BenchmarkLogger_Info_NoFields(b *testing.B) {
	buildBenchLogger(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Logger().Info("event")
	}
}

func BenchmarkLogger_Info_FiveFields(b *testing.B) {
	buildBenchLogger(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Logger().Info("event",
			Str("service", "payments"),
			Str("method", "GET"),
			Str("path", "/users/42"),
			Int("status", 200),
			Int("bytes", 1024),
		)
	}
}

func BenchmarkSugar_Infow_FiveFields(b *testing.B) {
	buildBenchLogger(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SugaredLogger().Infow("event",
			"service", "payments",
			"method", "GET",
			"path", "/users/42",
			"status", 200,
			"bytes", 1024,
		)
	}
}

func BenchmarkInfoCtx_WithExtractor(b *testing.B) {
	buildBenchLogger(b)
	SetContextFieldExtractor(func(ctx context.Context) []zapcore.Field {
		return []zapcore.Field{
			Str("request_id", "req-abc"),
			Str("trace_id", "trace-xyz"),
		}
	})
	b.Cleanup(func() { SetContextFieldExtractor(nil) })
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Info(ctx, "event", Int("status", 200))
	}
}
