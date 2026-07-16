package xlog

import (
	"context"
	"sync/atomic"
)

// contextFieldExtractor is set once at startup via SetContextFieldExtractor.
// Uses atomic pointer to avoid data races when tests call Set concurrently.
var contextFieldExtractor atomic.Pointer[func(context.Context) []Field]

// SetContextFieldExtractor sets the field extractor from context.
// Call once at startup. Examples: request-id, user-id, trace-id.
func SetContextFieldExtractor(fn func(context.Context) []Field) {
	contextFieldExtractor.Store(&fn)
}

func populateContextFields(ctx context.Context) []Field {
	if fn := contextFieldExtractor.Load(); fn != nil {
		return (*fn)(ctx)
	}
	return nil
}

// Convenience logging helpers with context
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
