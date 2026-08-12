## go-lib

Opinionated, small Go libraries for building production microservices. Each module ships independently — pick what you need, ignore the rest.

Design invariants across the stack:

- **Small surface**: one primary constructor per package (`Init`, `New`, ...) plus functional options. No hidden globals unless explicitly documented (`xlog.Logger()`, `otel.Tracer()`).
- **Context-first**: every ingress path threads `context.Context`; correlation/trace IDs flow through it, not through globals.
- **No cross-package import**: modules don't import each other. Wiring lives in your service `main`.
- **Boot once, defer shutdown**: `Init` returns a cleanup func; call it on `SIGTERM`.
- **Race-clean tests + benchmarks**: every module ships `bench_test.go` for regression tracking.

### Modules

Each module has its own `go.mod` and is tagged independently.

| Module | Path | Latest | What it does |
|---|---|---|---|
| [`config`](config/) | `github.com/viantonugroho11/go-lib/config` | v0.1.4 | Viper loader: Consul KV → file → ENV. Struct-tag-driven binding. |
| [`errors`](errors/) | `github.com/viantonugroho11/go-lib/errors` | v0.1.1 | Typed errors with stable `Code` + `Kind` + hot-reload dictionary for messages. |
| [`httpclient`](httpclient/) | `github.com/viantonugroho11/go-lib/httpclient` | v0.1.1 | Thin `http.Client`: base URL, retry, timeout, header defaults, correlation propagation. |
| [`httpserver`](httpserver/) | `github.com/viantonugroho11/go-lib/httpserver` | v0.1.2 | chi-based server: graceful shutdown, request ID, panic recover, timeouts, health/ready. |
| [`kafka`](kafka/) | `github.com/viantonugroho11/go-lib/kafka` | v0.3.3 | Sarama consumer + sync/async producers: DLQ, worker pool, OTel propagation, idempotent. |
| [`otel`](otel/) | `github.com/viantonugroho11/go-lib/otel` | v0.2.0 | OTLP bootstrap: traces + metrics + logs + runtime + Prometheus scrape + B3/Jaeger. |
| [`xlog`](xlog/) | `github.com/viantonugroho11/go-lib/xlog` | v0.1.2 | Zap logger: ENV/options config, rotation, ctx field extractor, sugar + typed API. |

### Requirements

Go 1.23+ minimum for `httpclient`, `httpserver`, `errors`, `xlog`, `otel`. `config` and `kafka` currently pin `go 1.25.0` after a security bump chain (grpc, otel, x/crypto); see the module `CHANGELOG.md`.

### Getting Started

Install just what you need:

```bash
go get github.com/viantonugroho11/go-lib/xlog@latest
go get github.com/viantonugroho11/go-lib/otel@latest
go get github.com/viantonugroho11/go-lib/httpserver@latest
# ...etc
```

Every module ships:

- `README.md` — quickstart, options, benchmarks, end-to-end wire notes.
- `example/` — runnable `main.go` (usually with `go run .` from the folder).
- `bench_test.go` — hot-path microbenchmarks; run with `go test -bench=. -benchmem`.

### Wiring the stack

The libraries are designed to compose without knowing about each other. Below is the canonical `main` for a service that uses all seven — copy, drop what you don't need.

```go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/viantonugroho11/go-lib/errors"
    "github.com/viantonugroho11/go-lib/httpserver"
    "github.com/viantonugroho11/go-lib/otel"
    "github.com/viantonugroho11/go-lib/xlog"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    // 1. Logging.
    _, cleanupLog := xlog.MustInitFromEnv()
    defer cleanupLog()

    // 2. Telemetry. Uses the global propagator picked up by httpserver / httpclient / kafka.
    mux := http.NewServeMux()
    shutdownOTel, err := otel.Init(ctx,
        otel.WithServiceName(os.Getenv("SERVICE_NAME")),
        otel.WithEnvironment(os.Getenv("ENV")),
        otel.WithEndpoint(os.Getenv("OTEL_ENDPOINT")),
        otel.WithRuntimeMetrics(),
        otel.WithPrometheusExporter(mux, "/metrics"),
        otel.WithErrorHandler(func(err error) {
            xlog.Logger().Error("otel", zap.Error(err))
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer shutdownOTel(context.Background())

    // 3. Log ↔ trace correlation. Feeds trace_id/span_id into every log line.
    xlog.SetContextFieldExtractor(func(ctx context.Context) []zapcore.Field {
        info, ok := otel.SpanContextInfo(ctx)
        if !ok {
            return nil
        }
        return []zapcore.Field{
            xlog.Str("trace_id", info.TraceID),
            xlog.Str("span_id", info.SpanID),
        }
    })

    // 4. Localised error messages.
    resolver, _ := errors.NewFileResolver("./messages",
        errors.WithDefaultLocale("en"),
        errors.WithReloadErrorHook(func(e error) { xlog.Logger().Error("dict reload", zap.Error(e)) }),
    )
    defer resolver.Close()
    errors.SetDefaultResolver(resolver)

    // 5. HTTP server with health, request ID, timeouts, structured request log.
    srv := httpserver.New(
        httpserver.WithAddr(":8080"),
        httpserver.WithRequestTimeout(5*time.Second),
        httpserver.WithLogger(func(ctx context.Context, m, p string, s int, d time.Duration) {
            xlog.Info(ctx, "http",
                xlog.Str("method", m), xlog.Str("path", p),
                xlog.Int("status", s), xlog.Dur("dur", d))
        }),
    )
    srv.Router().Mount("/", mux) // exposes /metrics from otel
    srv.Router().Get("/users/{id}", getUser)

    if err := srv.Run(ctx); err != nil {
        log.Fatal(err)
    }
}
```

### End-to-end trace propagation

Once `otel.Init` runs and sets the global `TextMapPropagator`, trace context flows automatically across:

- **HTTP ingress**: `httpserver.WithCorrelationHeader("traceparent", key)` extracts into ctx.
- **HTTP egress**: `httpclient.WithCorrelationHeader(key, "traceparent")` injects on outbound.
- **Kafka publish**: `kafka.Producer` / `AsyncProducer` inject `traceparent` + `tracestate` into message headers.
- **Kafka consume**: `kafka` extracts them before calling your `EventHandler`, so the handler ctx already carries the parent span.
- **Log correlation**: `xlog.SetContextFieldExtractor` + `otel.SpanContextInfo` adds `trace_id` / `span_id` to every log line.

No handler code needs to know about tracing.

### Backends

- **Traces**: Grafana Tempo (OTLP), Jaeger (`PropagatorJaeger`), Zipkin/Istio (`PropagatorB3`).
- **Metrics**: Grafana Mimir / Prometheus (OTLP remote-write or scrape via `WithPrometheusExporter`).
- **Logs**: Grafana Loki (via OTLP through the Collector) with `trace_id` derived field wired back to Tempo.
- **Recommended**: OpenTelemetry Collector in front, service targets a single endpoint. Backends swap via Collector config, not service redeploy.

See [`otel/README.md`](otel/README.md) for a full example collector config that fans out to Tempo + Mimir.

### Repository layout

```
config/       # v0.1.4  — Viper config loader
errors/       # v0.1.1  — typed errors + i18n dictionary
httpclient/   # v0.1.1  — retry / correlation http client
httpserver/   # v0.1.2  — chi + graceful shutdown + middleware
kafka/        # v0.3.3  — consumer + producers + OTel + DLQ
otel/         # v0.2.0  — telemetry bootstrap
xlog/         # v0.1.2  — zap logger
CLAUDE.md     # architecture notes for AI assistants
```

Each folder has a `README.md` and `CHANGELOG.md` — start there.

### Contributing

Every module must remain independently buildable (`cd <module> && go build ./... && go test -race ./...`). No cross-module imports. Benchmarks stay in `bench_test.go`.

Commit messages follow conventional format: `<type>(<module>): message`.

Tags are per-module: `<module>/vX.Y.Z`.

### License

MIT.
