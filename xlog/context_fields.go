package xlog

import (
	"context"
)

// contextFieldExtractor allows users to inject a context-based field extractor.
var contextFieldExtractor func(context.Context) []Field

// SetContextFieldExtractor sets the field extractor from context.
// Examples: request-id, user-id, trace-id, etc.
func SetContextFieldExtractor(fn func(context.Context) []Field) {
	contextFieldExtractor = fn
}

func populateContextFields(ctx context.Context) []Field {
	if contextFieldExtractor != nil {
		return contextFieldExtractor(ctx)
	}
	// default: no additional fields
	return []Field{
		// optional example: add standardized fields here if needed
		// Str("logger", "xlog"),
	}
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
