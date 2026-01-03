package xlog

import (
	"context"
)

// contextFieldExtractor memungkinkan pengguna meng-inject extractor field berbasis context.
var contextFieldExtractor func(context.Context) []Field

// SetContextFieldExtractor menetapkan extractor field dari context.
// Contoh: request-id, user-id, trace-id, dsb.
func SetContextFieldExtractor(fn func(context.Context) []Field) {
	contextFieldExtractor = fn
}

func populateContextFields(ctx context.Context) []Field {
	if contextFieldExtractor != nil {
		return contextFieldExtractor(ctx)
	}
	// default: tidak ada field tambahan
	return []Field{
		// contoh opsional: tambahkan level agar konsisten (dilepas agar netral)
		// Str("logger", "xlog"),
	}
}

// Convenience loggers dengan context
func Info(ctx context.Context, message string, fields ...Field) {
	Logger().With(populateContextFields(ctx)...).Info(message, fields...)
}

func Debug(ctx context.Context, message string, fields ...Field) {
	Logger().With(populateContextFields(ctx)...).Debug(message, fields...)
}

func Warn(ctx context.Context, message string, fields ...Field) {
	Logger().With(populateContextFields(ctx)...).Warn(message, fields...)
}

func DPanic(ctx context.Context, message string, fields ...Field) {
	Logger().With(populateContextFields(ctx)...).DPanic(message, fields...)
}

func Panic(ctx context.Context, message string, fields ...Field) {
	Logger().With(populateContextFields(ctx)...).Panic(message, fields...)
}

func Fatal(ctx context.Context, message string, fields ...Field) {
	Logger().With(populateContextFields(ctx)...).Fatal(message, fields...)
}
