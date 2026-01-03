package xlog

import (
	"context"
	"time"

	"go.uber.org/zap/zapcore"
)

// Option adalah fungsi opsional untuk mengubah konfigurasi logger.
type Option func(cfg *Config)

// OutputMode menentukan ke mana log ditulis.
type OutputMode string

const (
	// OutputStdout menulis log ke stdout.
	OutputStdout OutputMode = "stdout"
	// OutputStderr menulis log ke stderr.
	OutputStderr OutputMode = "stderr"
	// OutputFile menulis log ke file dengan dukungan rotation.
	OutputFile OutputMode = "file"
)

// FileRotation mengatur kebijakan rotasi file log.
type FileRotation struct {
	Path       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// SamplingConfig untuk mengurangi spam log di trafik tinggi.
type SamplingConfig struct {
	Enabled    bool
	Initial    int
	Thereafter int
}

// Config adalah konfigurasi utama logger.
type Config struct {
	Level             string
	Format            string // "json" atau "console"
	Output            OutputMode
	File              FileRotation
	AddCaller         bool
	Development       bool
	Sampling          SamplingConfig
	StacktraceLevel   string // contoh: "error"
	TimeFieldKey      string // default "ts"
	TimeEncoderLayout string // contoh: time.RFC3339Nano, kosong => zap default
}

// defaultConfig mengembalikan konfigurasi produksi yang "optimal".
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

// WithLevel mengatur level log (debug, info, warn, error, dpanic, panic, fatal).
func WithLevel(level string) Option {
	return func(cfg *Config) {
		cfg.Level = level
	}
}

// WithJSONFormat mengatur format JSON.
func WithJSONFormat() Option {
	return func(cfg *Config) {
		cfg.Format = "json"
	}
}

// WithConsoleFormat mengatur format console (human-readable).
func WithConsoleFormat() Option {
	return func(cfg *Config) {
		cfg.Format = "console"
	}
}

// WithOutputStdout mengatur output ke stdout.
func WithOutputStdout() Option {
	return func(cfg *Config) {
		cfg.Output = OutputStdout
	}
}

// WithOutputStderr mengatur output ke stderr.
func WithOutputStderr() Option {
	return func(cfg *Config) {
		cfg.Output = OutputStderr
	}
}

// WithOutputFile mengatur output ke file dengan detail rotasi.
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

// WithAddCaller menambahkan informasi caller (file:line) di log.
func WithAddCaller(enable bool) Option {
	return func(cfg *Config) {
		cfg.AddCaller = enable
	}
}

// WithDevelopment mengaktifkan mode development (stacktrace lebih agresif, dsb).
func WithDevelopment(enable bool) Option {
	return func(cfg *Config) {
		cfg.Development = enable
	}
}

// WithSampling mengatur sampling log.
func WithSampling(initial, thereafter int) Option {
	return func(cfg *Config) {
		cfg.Sampling.Enabled = true
		cfg.Sampling.Initial = initial
		cfg.Sampling.Thereafter = thereafter
	}
}

// WithoutSampling menonaktifkan sampling.
func WithoutSampling() Option {
	return func(cfg *Config) {
		cfg.Sampling.Enabled = false
	}
}

// WithStacktraceLevel mengatur level stacktrace (mis: "error").
func WithStacktraceLevel(level string) Option {
	return func(cfg *Config) {
		cfg.StacktraceLevel = level
	}
}

// WithTimeEncoderLayout mengatur layout waktu.
func WithTimeEncoderLayout(layout string) Option {
	return func(cfg *Config) {
		cfg.TimeEncoderLayout = layout
	}
}

// WithTimeFieldKey mengatur nama field waktu.
func WithTimeFieldKey(key string) Option {
	return func(cfg *Config) {
		cfg.TimeFieldKey = key
	}
}

type Field = zapcore.Field

// Error melakukan logging error pada logger global dengan dukungan field dari context.
func Error(ctx context.Context, message string, fields ...Field) {
	Logger().With(populateContextFields(ctx)...).Error(message, fields...)
}
