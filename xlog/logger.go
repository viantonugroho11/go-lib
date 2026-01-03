package xlog

import (
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Init menginisialisasi logger global berbasis zap sesuai Option yang diberikan.
// Mengembalikan logger, fungsi cleanup, dan error jika ada.
// Cleanup akan memanggil Sync() dan mengembalikan konfigurasi stdlog seperti semula.
func Init(options ...Option) (*zap.Logger, func(), error) {
	cfg := defaultConfig()
	for _, opt := range options {
		opt(cfg)
	}

	level := parseLevel(cfg.Level)
	stacktraceLevel := parseLevel(cfg.StacktraceLevel)

	encoder := buildEncoder(cfg)
	writer := buildWriteSyncer(cfg)

	core := zapcore.NewCore(encoder, writer, level)
	if cfg.Sampling.Enabled {
		core = zapcore.NewSamplerWithOptions(
			core,
			/*tick=*/ 0, // diabaikan oleh zap; sampler berdasar count
			cfg.Sampling.Initial,
			cfg.Sampling.Thereafter,
		)
	}

	zapOptions := []zap.Option{
		zap.AddStacktrace(stacktraceLevel),
	}
	if cfg.AddCaller {
		zapOptions = append(zapOptions, zap.AddCaller())
	}
	if cfg.Development {
		zapOptions = append(zapOptions, zap.Development())
	}

	logger := zap.New(core, zapOptions...)

	undo := zap.RedirectStdLog(logger)
	restoreGlobals := zap.ReplaceGlobals(logger)
	cleanup := func() {
		_ = logger.Sync()
		undo()
		restoreGlobals()
	}
	return logger, cleanup, nil
}

// MustInit sama seperti Init namun panic jika terjadi error.
func MustInit(options ...Option) (*zap.Logger, func()) {
	l, c, err := Init(options...)
	if err != nil {
		panic(err)
	}
	return l, c
}

// L mengembalikan global logger (non-sugared).
func L() *zap.Logger {
	return zap.L()
}

// S mengembalikan global sugared logger.
func S() *zap.SugaredLogger {
	return zap.S()
}

// Logger mengembalikan logger global (nama formal untuk L).
func Logger() *zap.Logger {
	return zap.L()
}

// SugaredLogger mengembalikan sugared logger global (nama formal untuk S).
func SugaredLogger() *zap.SugaredLogger {
	return zap.S()
}

func parseLevel(lvl string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(lvl)) {
	case "debug":
		return zap.DebugLevel
	case "info", "":
		return zap.InfoLevel
	case "warn", "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	case "dpanic":
		return zap.DPanicLevel
	case "panic":
		return zap.PanicLevel
	case "fatal":
		return zap.FatalLevel
	default:
		return zap.InfoLevel
	}
}

func buildEncoder(cfg *Config) zapcore.Encoder {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = cfg.TimeFieldKey
	if cfg.TimeEncoderLayout != "" {
		encCfg.EncodeTime = zapcore.TimeEncoderOfLayout(cfg.TimeEncoderLayout)
	}
	encCfg.EncodeDuration = zapcore.SecondsDurationEncoder
	encCfg.EncodeLevel = zapcore.LowercaseLevelEncoder
	encCfg.EncodeCaller = zapcore.ShortCallerEncoder

	if strings.EqualFold(cfg.Format, "console") {
		return zapcore.NewConsoleEncoder(encCfg)
	}
	return zapcore.NewJSONEncoder(encCfg)
}

func buildWriteSyncer(cfg *Config) zapcore.WriteSyncer {
	switch cfg.Output {
	case OutputStderr:
		return zapcore.Lock(os.Stderr)
	case OutputFile:
		l := &lumberjack.Logger{
			Filename:   cfg.File.Path,
			MaxSize:    cfg.File.MaxSizeMB,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAgeDays,
			Compress:   cfg.File.Compress,
		}
		return zapcore.AddSync(&writerWithSync{Writer: l})
	case OutputStdout:
		fallthrough
	default:
		return zapcore.Lock(os.Stdout)
	}
}

// writerWithSync memastikan writer yang tidak implement Sync tetap aman dipakai.
type writerWithSync struct {
	io.Writer
}

func (w writerWithSync) Sync() error { return nil }
