# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Structure

Three independently versioned Go modules, each with its own `go.mod`:

```
config/   — github.com/viantonugroho11/go-lib/config   (Go 1.23+)
kafka/    — github.com/viantonugroho11/go-lib/kafka     (Go 1.23+)
xlog/     — github.com/viantonugroho11/go-lib/xlog      (Go 1.23+)
```

Each module is released separately. Changes to one module do not require bumping the others.

## Commands

All commands must be run from within the relevant module directory (e.g., `cd kafka/`).

```bash
# Test
go test ./...

# Test single package with verbose
go test -v -run TestName ./...

# Test with race detector
go test -race ./...

# Build (verify compilation)
go build ./...

# Vet
go vet ./...

# Tidy
go mod tidy
```

There is no top-level Makefile or workspace file. Running `go test ./...` from the repo root will fail because there is no root `go.mod`.

## Architecture

### kafka

- **`model.go`** — Public types: `Header`, `Progress`, `ProgressStatus`, `EventHandler[E]`, `EventProducer[E]`, `EventAndHeader[E]`. Central contract for consumers and producers.
- **`consumer.go`** — `NewConsumer[E]` returns `Consumer` interface (`Start`/`Close`). Internally: sarama `ConsumerGroup` + two goroutines (consume loop + error drain). `group.Close()` must precede `wg.Wait()` to avoid deadlock.
- **`handler.go`** — `adaptEventHandler[E]` bridges `EventHandler[E]` to the internal `messageHandler` func. Handles JSON decode, header filtering, and `Progress` → error mapping.
- **`producer.go`** — `Producer[T]` wraps sarama `SyncProducer`. Implements `EventProducer[T]`. Key set via `WithKey` (fixed) or `WithKeyFunc[T]` (per-message). `ctx` is accepted but not propagated (sarama limitation).
- **`options.go`** / **`producer_options.go`** — Functional options for each type. Use `With*Consumer*` prefix for consumer-specific options. `WithKey`/`WithKeyFunc` are producer-only.

**Key invariant:** `ProgressError` prevents offset commit (message retried). `ProgressSuccess`/`ProgressSkip`/`ProgressDrop` all commit. `SetError()` on `Progress` sets both `Err` and `Status=ProgressError`.

### config

- **`config.go`** — `ViperLoader` wraps `*viper.Viper`. `New(envPrefix, consulKey, consulURL, opts...)` builds it. `Load(cfg interface{})` tries Consul first (retries up to `remoteMaxAttempt`), falls back to file + env.
- Package name is `config_load` (not `config`). Import as `config_load "github.com/viantonugroho11/go-lib/config"`.
- ENV overrides use `ENVPREFIX_FIELD_SUBFIELD` pattern (dots replaced with underscores). Controlled by `envPrefix` passed to `New`.
- Default struct tag for mapping is `"json"`. Override with `WithStructTagName("mapstructure")`.

### xlog

- **`logger.go`** — `Init(opts...)` builds a zap logger, sets it as global via `zap.ReplaceGlobals`, and returns `(logger, cleanup, error)`. Always call `cleanup()` on shutdown.
- **`env.go`** — `InitFromEnv()` reads `LOG_*` env vars and calls `Init`. Use in services.
- **`context_fields.go`** — `SetContextFieldExtractor(fn)` registers a function that pulls fields (trace ID, user ID, etc.) from `context.Context`. Called once at startup. Package-level log helpers (`Info`, `Error`, `Warn`, etc.) all pass ctx through this extractor.
- **`fields.go`** — Thin wrappers over `zap.*` field constructors for ergonomic imports.
- Global logger accessed via `xlog.Logger()` or `xlog.L()` (same thing, two names).

## Key Patterns

**Consumer implementation:**
```go
type MyHandler struct{}
func (h MyHandler) Name() string { return "my_handler" }
func (h MyHandler) Handle(ctx context.Context, evt MyEvent, headers ...kafka.Header) kafka.Progress {
    // return kafka.Progress{Status: kafka.ProgressSuccess}
    // or p := kafka.Progress{}; p.SetError(err); return p
}
```

**Producer satisfies EventProducer[T]:**
```go
var _ kafka.EventProducer[MyEvent] = (*kafka.Producer[MyEvent])(nil)
```

**WithKeyFunc type must match Producer type parameter exactly:**
```go
kafka.WithKeyFunc(func(e MyEvent) []byte { return []byte(e.ID) })
```
