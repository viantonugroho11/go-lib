# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Structure

Independently versioned Go modules, each with its own `go.mod`:

```
config/       — github.com/viantonugroho11/go-lib/config       (Go 1.23+)
httpclient/   — github.com/viantonugroho11/go-lib/httpclient   (Go 1.23+)
httpserver/   — github.com/viantonugroho11/go-lib/httpserver   (Go 1.23+)
kafka/        — github.com/viantonugroho11/go-lib/kafka        (Go 1.23+)
xlog/         — github.com/viantonugroho11/go-lib/xlog         (Go 1.23+)
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
- **`consumer.go`** — `NewConsumer[E](brokers, groupID, topic string, handler, opts...)` returns `Consumer` (`Start`/`Close`). One consumer, one topic. Internally: sarama `ConsumerGroup` + two goroutines (consume loop + error drain). `group.Close()` must precede `wg.Wait()` to avoid deadlock. Serial path (`concurrency=1`) calls `processOne` per message; pool path (`concurrency>1`) fans out to N workers + contiguous-commit tracker so a hole never advances the commit past it.
- **`async_producer.go`** — `AsyncProducer[T]` wraps sarama `AsyncProducer`. `Publish` is non-blocking (enqueue only); delivery outcome fires via `WithAsyncCallback[T](fn)` on a drain goroutine. Both `Successes` and `Errors` channels are forced on to power the callback. `Close` flushes in-flight messages.
- **`error_strategy.go`** — `ErrorStrategy` = `ErrorSkip` (default, log + advance), `ErrorBlock` (retry with `BlockBackoff` until success or ctx cancel), `ErrorDeadLetter` (route to `DeadLetterFunc`, then advance; falls back to skip on DLQ publish failure).
- **`handler.go`** — `adaptEventHandler[E]` bridges `EventHandler[E]` to the internal `messageHandler` func. Extracts OTel trace context from all headers before filtering, then decodes JSON, filters headers by `WithHeaderKeys`, and maps `Progress` → `messageResult`.
- **`trace.go`** — `injectTrace`/`extractTrace` use the global `otel.TextMapPropagator` to round-trip `traceparent`/`tracestate` across producer → consumer. No-op when no propagator is configured.
- **`logger.go`** — `Logger` interface (`Errorf`/`Infof`); default `stderrLogger`. Wire `xlog` (or any adapter) via `WithLogger`.
- **`producer.go`** — `Producer[T]` wraps sarama `SyncProducer`. Implements `EventProducer[T]`. Key set via `WithKey` (fixed) or `WithKeyFunc[T]` (per-message). Encoder overridable via `WithEncoder[T]`. Idempotent by default (`acks=all`, `MaxOpenRequests=1`). `ctx` is accepted for trace injection but is not propagated into sarama's `SendMessage` wait (bounded by `Producer.Timeout`).
- **`options.go`** / **`producer_options.go`** — Functional options for each type. Consumer-only: `WithHeaderKeys`, `WithLogger`, `WithErrorStrategy`, `WithDeadLetter`, `WithBlockOnError`, `With*Consumer*`, TLS, SASL, offsets. Producer-only: `WithKey`, `WithKeyFunc[T]`, `WithEncoder[T]`, `WithIdempotent(bool)`, `WithAcks`, `WithRetry*`, `WithCompression`, `WithTimeout`.

**Key invariants:**
- `ProgressError` handling is determined by `ErrorStrategy` (formerly silent-drop; see kafka/v0.2.0 CHANGELOG for the offset-commit bug fixed here).
- `ProgressSuccess`/`ProgressSkip`/`ProgressDrop` all commit.
- `SetError()` on `Progress` sets both `Err` and `Status=ProgressError`.
- OTel trace context is injected on publish and extracted on consume automatically; no handler changes required.

### config

- **`config.go`** — `ViperLoader` wraps `*viper.Viper`. `New(envPrefix, consulKey, consulURL, opts...)` builds it. `Load(cfg interface{})` tries Consul first (retries up to `remoteMaxAttempt`), falls back to file + env.
- Package name is `config_load` (not `config`). Import as `config_load "github.com/viantonugroho11/go-lib/config"`.
- ENV overrides use `ENVPREFIX_FIELD_SUBFIELD` pattern (dots replaced with underscores). Controlled by `envPrefix` passed to `New`.
- Default struct tag for mapping is `"json"`. Override with `WithStructTagName("mapstructure")`.

### httpserver

- **`server.go`** — `New(opts...)` returns `*Server` wrapping chi router + `*http.Server`. `Run(ctx)` blocks, gracefully shuts down when ctx cancels (bounded by `WithShutdownTimeout`, default 15s). `Router()` exposes `chi.Router` for route registration; `Handler()` returns the raw `http.Handler` for `httptest`.
- **`middleware.go`** — Built-ins: request ID (reads/generates + echoes header), correlation headers (ingress header → ctx), panic recover, request timeout, logging hook. All applied in order in `New`.
- **`options.go`** — Functional options. `WithLogger(LoggerFunc)` wires request completion logging (fires with method, path, status, duration) — pair with `xlog`. `WithCorrelationHeader(header, ctxKey)` mirrors `httpclient.WithCorrelationHeader` for end-to-end propagation. `WithReadyCheck(fn)` plugs a readiness probe for `/readyz`.
- **`context.go`** — `CtxKeyRequestID` default ctx key. `RequestIDFromContext(ctx)` returns the stored ID or `""`.
- Framework: `github.com/go-chi/chi/v5`. Server does not lock consumer into chi wrappers — full chi API available via `Router()`.

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
