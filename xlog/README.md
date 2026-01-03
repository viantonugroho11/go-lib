## xlog

Lightweight logging package built on `zap` with:
- Configuration via options or ENV
- Output to stdout/stderr/file (with rotation)
- Field helpers (Str, Int, Any, Err, etc.)
- Context field extractor support (e.g., request-id)
- Formal global accessors: `Logger()` and `SugaredLogger()` (aliases: `L()` and `S()`)

### Installation
- Import according to your module path (in this repo the module is named `xlog`):

```go
import xlog "xlog"
```

### Quick Start (ENV)
```go
_, cleanup := xlog.MustInitFromEnv()
defer cleanup()

xlog.SugaredLogger().Infow("service started", "version", "1.0.0")
xlog.Logger().Info("ready", xlog.Str("module", "api"))
```

Supported ENV:
- LOG_LEVEL: debug|info|warn|error|dpanic|panic|fatal (default: info)
- LOG_ENCODING: json|console (default: json)  ← prefer this instead of LOG_FORMAT
- LOG_OUTPUT: stdout|stderr|file (default: stdout)
- LOG_FILE_PATH (default: app.log)
- LOG_FILE_MAX_SIZE_MB (default: 100)
- LOG_FILE_MAX_BACKUPS (default: 3)
- LOG_FILE_MAX_AGE_DAYS (default: 28)
- LOG_FILE_COMPRESS (default: false)
- LOG_ADD_CALLER (default: true)
- LOG_DEVELOPMENT (default: false)
- LOG_SAMPLING (default: true)
- LOG_SAMPLING_INITIAL (default: 100)
- LOG_SAMPLING_THEREAFTER (default: 100)
- LOG_STACKTRACE_LEVEL (default: error)

Note: If you previously used `LOG_FORMAT`, it is still supported; the recommended key is `LOG_ENCODING`.

### Configuration via Options (programmatic)
```go
_, cleanup := xlog.MustInit(
	xlog.WithLevel("debug"),
	xlog.WithConsoleFormat(),  // or xlog.WithJSONFormat()
	xlog.WithStdout(),         // or xlog.WithStderr() / xlog.WithOutputFile("app.log", 100, 3, 28, true)
	xlog.WithSampling(100, 100),
	xlog.WithStacktraceLevel("error"),
	xlog.WithAddCaller(true),
)
defer cleanup()
```

### Field Helpers
```go
xlog.Logger().Info("payment processed",
	xlog.Str("order_id", "ord-1"),
	xlog.Int("amount", 12500),
)
```

Available: `Str`, `Bool`, `Int`, `Int64`, `Float64`, `Time`, `Dur`, `Any`, `Err`, `NamedError`, `Object`, `Stringer`.

### Context Field Extractor
You can inject fields from `context.Context` (e.g., `request_id`, `user_id`, `trace_id`) centrally:
```go
xlog.SetContextFieldExtractor(func(ctx context.Context) []xlog.Field {
	if v := ctx.Value("request_id"); v != nil {
		return []xlog.Field{xlog.Str("request_id", v.(string))}
	}
	return nil
})

ctx := context.WithValue(context.Background(), "request_id", "req-123")
xlog.Error(ctx, "failed to process", xlog.Err(errors.New("boom")))
```

### Global Access
- `Logger()` is the formal alias for `L()` → non-sugared (structured) logger
- `SugaredLogger()` is the formal alias for `S()` → sugared (key-value) logger

Examples:
```go
xlog.Logger().Info("structured", xlog.Str("k", "v"))
xlog.SugaredLogger().Infow("kv-style", "k", "v")
```

### Output File + Rotation
If you use `WithOutputFile(...)`, this package uses `lumberjack` for log rotation.


