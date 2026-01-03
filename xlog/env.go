package xlog

import (
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// InitFromEnv menginisialisasi logger dengan konfigurasi dari ENV:
// - LOG_LEVEL: debug|info|warn|error|dpanic|panic|fatal (default: info)
// - LOG_FORMAT: json|console (default: json)
// - LOG_OUTPUT: stdout|stderr|file (default: stdout)
// - LOG_FILE_PATH: path file (default: app.log)
// - LOG_FILE_MAX_SIZE_MB: int (default: 100)
// - LOG_FILE_MAX_BACKUPS: int (default: 3)
// - LOG_FILE_MAX_AGE_DAYS: int (default: 28)
// - LOG_FILE_COMPRESS: bool (default: false)
// - LOG_ADD_CALLER: bool (default: true)
// - LOG_DEVELOPMENT: bool (default: false)
// - LOG_SAMPLING: bool (default: true)
// - LOG_SAMPLING_INITIAL: int (default: 100)
// - LOG_SAMPLING_THEREAFTER: int (default: 100)
// - LOG_STACKTRACE_LEVEL: error|warn|debug|... (default: error)
func InitFromEnv() (*zap.Logger, func(), error) {
	opts := []Option{}

	level := getEnv("LOG_LEVEL", "info")
	opts = append(opts, WithLevel(level))

	format := strings.ToLower(getEnv("LOG_FORMAT", "json"))
	switch format {
	case "console":
		opts = append(opts, WithConsoleFormat())
	default:
		opts = append(opts, WithJSONFormat())
	}

	output := strings.ToLower(getEnv("LOG_OUTPUT", "stdout"))
	switch output {
	case "stderr":
		opts = append(opts, WithOutputStderr())
	case "file":
		path := getEnv("LOG_FILE_PATH", "app.log")
		maxSize := getEnvInt("LOG_FILE_MAX_SIZE_MB", 100)
		maxBackups := getEnvInt("LOG_FILE_MAX_BACKUPS", 3)
		maxAge := getEnvInt("LOG_FILE_MAX_AGE_DAYS", 28)
		compress := getEnvBool("LOG_FILE_COMPRESS", false)
		opts = append(opts, WithOutputFile(path, maxSize, maxBackups, maxAge, compress))
	default:
		opts = append(opts, WithOutputStdout())
	}

	addCaller := getEnvBool("LOG_ADD_CALLER", true)
	opts = append(opts, WithAddCaller(addCaller))

	dev := getEnvBool("LOG_DEVELOPMENT", false)
	opts = append(opts, WithDevelopment(dev))

	if getEnvBool("LOG_SAMPLING", true) {
		opts = append(opts, WithSampling(
			getEnvInt("LOG_SAMPLING_INITIAL", 100),
			getEnvInt("LOG_SAMPLING_THEREAFTER", 100),
		))
	} else {
		opts = append(opts, WithoutSampling())
	}

	stackLvl := getEnv("LOG_STACKTRACE_LEVEL", "error")
	opts = append(opts, WithStacktraceLevel(stackLvl))

	return Init(opts...)
}

// MustInitFromEnv panic jika inisialisasi gagal.
func MustInitFromEnv() (*zap.Logger, func()) {
	l, c, err := InitFromEnv()
	if err != nil {
		panic(err)
	}
	return l, c
}

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "t", "yes", "y":
			return true
		case "0", "false", "f", "no", "n":
			return false
		}
	}
	return def
}
