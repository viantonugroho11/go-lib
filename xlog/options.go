package xlog

import (
	"context"
	"time"

	"go.uber.org/zap/zapcore"
)

// Option is a functional option that mutates logger configuration.
type Option func(cfg *Config)

// OutputMode determines where logs are written.
type OutputMode string

const (
	// OutputStdout writes logs to stdout.
	OutputStdout OutputMode = "stdout"
	// OutputStderr writes logs to stderr.
	OutputStderr OutputMode = "stderr"
	// OutputFile writes logs to a file with rotation support.
	OutputFile OutputMode = "file"
)

// FileRotation configures file log rotation policy.
type FileRotation struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// SamplingConfig reduces log spam under high traffic.
type SamplingConfig struct {
	Enabled    bool
	Initial    int
	Thereafter int
}

// Config is the primary logger configuration.
type Config struct {
	Level             string
	Format            string // "json" or "console"
	Output            OutputMode
	File              FileRotation
	AddCaller         bool
	Development       bool
	Sampling          SamplingConfig
	StacktraceLevel   string // e.g. "error"
	TimeFieldKey      string // default "ts"
	TimeEncoderLayout string // e.g. time.RFC3339Nano, empty => zap default
}

// defaultConfig returns an "optimal" production configuration.
func defaultConfig() *Config {
	return &Config{
		Level:  "info",
		Format: "json",
		Output: OutputStdout,
		File: FileRotation{
			Path:       "app.log",
			MaxSizeMB:  100,
			MaxBackups: 3,
			MaxAgeDays: 28,
			Compress:   false,
		},
		AddCaller:   true,
		Development: false,
		Sampling: SamplingConfig{
			Enabled:    true,
			Initial:    100,
			Thereafter: 100,
		},
		StacktraceLevel:   "error",
		TimeFieldKey:      "ts",
		TimeEncoderLayout: time.RFC3339Nano,
	}
}

// WithLevel sets log level (debug, info, warn, error, dpanic, panic, fatal).
func WithLevel(level string) Option {
	return func(cfg *Config) {
		cfg.Level = level
	}
}

// WithJSONFormat sets JSON encoding.
func WithJSONFormat() Option {
	return func(cfg *Config) {
		cfg.Format = "json"
	}
}

// WithConsoleFormat sets console (human-readable) encoding.
func WithConsoleFormat() Option {
	return func(cfg *Config) {
		cfg.Format = "console"
	}
}

// WithOutputStdout sets output to stdout.
func WithOutputStdout() Option {
	return func(cfg *Config) {
		cfg.Output = OutputStdout
	}
}

// WithOutputStderr sets output to stderr.
func WithOutputStderr() Option {
	return func(cfg *Config) {
		cfg.Output = OutputStderr
	}
}

// WithOutputFile sets output to a file with rotation details.
func WithOutputFile(path string, maxSizeMB, maxBackups, maxAgeDays int, compress bool) Option {
	return func(cfg *Config) {
		cfg.Output = OutputFile
		cfg.File.Path = path
		cfg.File.MaxSizeMB = maxSizeMB
		cfg.File.MaxBackups = maxBackups
		cfg.File.MaxAgeDays = maxAgeDays
		cfg.File.Compress = compress
	}
}

// WithAddCaller toggles caller info (file:line).
func WithAddCaller(enable bool) Option {
	return func(cfg *Config) {
		cfg.AddCaller = enable
	}
}

// WithDevelopment enables development mode (more aggressive stacktraces, etc.).
func WithDevelopment(enable bool) Option {
	return func(cfg *Config) {
		cfg.Development = enable
	}
}

// WithSampling configures log sampling.
func WithSampling(initial, thereafter int) Option {
	return func(cfg *Config) {
		cfg.Sampling.Enabled = true
		cfg.Sampling.Initial = initial
		cfg.Sampling.Thereafter = thereafter
	}
}

// WithoutSampling disables sampling.
func WithoutSampling() Option {
	return func(cfg *Config) {
		cfg.Sampling.Enabled = false
	}
}

// WithStacktraceLevel sets the stacktrace level (e.g., "error").
func WithStacktraceLevel(level string) Option {
	return func(cfg *Config) {
		cfg.StacktraceLevel = level
	}
}

// WithTimeEncoderLayout sets the time layout.
func WithTimeEncoderLayout(layout string) Option {
	return func(cfg *Config) {
		cfg.TimeEncoderLayout = layout
	}
}

// WithTimeFieldKey sets the time field key.
func WithTimeFieldKey(key string) Option {
	return func(cfg *Config) {
		cfg.TimeFieldKey = key
	}
}

type Field = zapcore.Field

// Error logs an error using the global logger with context-derived fields.
func Error(ctx context.Context, message string, fields ...Field) {
	Logger().With(populateContextFields(ctx)...).Error(message, fields...)
}
