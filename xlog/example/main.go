package main

import (
	"context"
	"errors"
	"time"
	xlog "xlog"
)

func main() {
	_, cleanup := xlog.MustInitFromEnv()
	defer cleanup()

	// Inject context field extractor
	xlog.SetContextFieldExtractor(func(ctx context.Context) []xlog.Field {
		if v := ctx.Value("request_id"); v != nil {
			return []xlog.Field{xlog.Str("request_id", v.(string))}
		}
		return nil
	})

	ctx := context.WithValue(context.Background(), "request_id", "req-123")

		// example of sugared (kv-style)
	xlog.SugaredLogger().Infow("service started", "module", "example", "feature", "xlog")

	// example of structured log
	xlog.Logger().Info("structured log", xlog.Str("module", "example"))

	// example of any log
	xlog.Logger().Info("any log", xlog.Any("any", "any"))

	// example of time log
	xlog.Logger().Info("time log", xlog.Time("time", time.Now()))

	// example of duration log
	xlog.Logger().Info("duration log", xlog.Dur("duration", 1*time.Second))

	// example of error log
	err := errors.New("something went wrong")
	xlog.Error(ctx, "failed to process", xlog.Err(err))
}
