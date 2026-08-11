package kafka

import (
	"context"
	"fmt"
	"os"
)

// Logger is the minimal logging surface used by consumer/producer.
// Wire xlog (or any zap/slog wrapper) via WithLogger.
type Logger interface {
	Errorf(ctx context.Context, format string, args ...any)
	Infof(ctx context.Context, format string, args ...any)
}

// stderrLogger is the default Logger; writes to stderr.
type stderrLogger struct{}

func (stderrLogger) Errorf(_ context.Context, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "kafka ERROR: "+format+"\n", args...)
}
func (stderrLogger) Infof(_ context.Context, format string, args ...any) {
	fmt.Fprintf(os.Stderr, "kafka INFO: "+format+"\n", args...)
}
