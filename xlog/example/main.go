package main

import (
	"context"
	"errors"
	xlog "xlog"
)

func main() {
	_, cleanup := xlog.MustInitFromEnv()
	defer cleanup()

	// Inject field dari context (opsional)
	xlog.SetContextFieldExtractor(func(ctx context.Context) []xlog.Field {
		if v := ctx.Value("request_id"); v != nil {
			return []xlog.Field{xlog.Str("request_id", v.(string))}
		}
		return nil
	})

	ctx := context.WithValue(context.Background(), "request_id", "req-123")

	// Contoh pemakaian sugared (kv-style)
	xlog.SugaredLogger().Infow("service started", "module", "example", "feature", "xlog")

	// Contoh pemakaian structured
	xlog.Logger().Info("structured log", xlog.Str("module", "example"))

	// Contoh pemakaian fungsi berbasis context
	err := errors.New("something went wrong")
	xlog.Error(ctx, "failed to process", xlog.Err(err))
}
