# Changelog

## v0.2.0 - 2026-08-12

### Features (additive; no API break)

- **Logs pipeline** — `Init` now wires a `sdklog.LoggerProvider` by default (OTLP or stdout). `Logger(name)` returns a `log.Logger` from the global provider. Opt out with `WithoutLogs()`. Bridge into `xlog`/`slog`/`zap` at your own layer; otel does not import those.
- **Runtime metrics** — `WithRuntimeMetrics()` enables `contrib/instrumentation/runtime` (goroutines, GC, heap, memory). Free RED-ish signals for every service.
- **Prometheus scrape reader** — `WithPrometheusExporter(mux, path)` attaches a Prometheus-format `/metrics` endpoint alongside the OTLP push pipeline. Same measurements, two backends.
- **Stdout exporter** — `WithStdoutExporter()` routes traces + metrics + logs to stdout instead of OTLP. Wins over the OTLP path. Use in tests + local dev without a collector.
- **TLS config** — `WithTLSConfig(*tls.Config)` for OTLP transport (CA cert, mTLS). Wins over `WithInsecure`. Applied to trace + metric + log exporters uniformly.
- **Custom span processor** — `WithSpanProcessor(sdktrace.SpanProcessor)` appends processors alongside the built-in batch processor. Wire debug printers, PII redactors, or custom taggers.
- **B3 + Jaeger propagator helpers** — `PropagatorB3()` and `PropagatorJaeger()`. Compose with `TraceContext` via `WithPropagators` for cross-service interop.

### Env

- `GO_LIB_OTEL_DISABLE_LOGS` — skip logs pipeline.
- `GO_LIB_OTEL_RUNTIME_METRICS` — enable runtime metrics.
- `GO_LIB_OTEL_STDOUT_EXPORTER` — route to stdout.

### Tests

- 8 → 15 tests, race-clean. New coverage: logs pipeline install + shutdown restores noop, `WithoutLogs`, stdout exporter path, Prometheus scrape endpoint returns real body containing the emitted counter, TLS config overrides insecure, extra span processor runs, B3 + Jaeger propagator field names, composite propagator with B3.


## v0.1.2 - 2026-08-12

### Bug Fixes

- **metric interval bug**: metric periodic-reader interval was tied to `WithBatchTimeout` (default 5s). Now decoupled — new `WithMetricInterval(d)` option (default 30s). Fixes cases where users wanted fast trace flushes (2s) but longer metric intervals (30–60s).

### Features

- **`WithErrorHandler(fn)`** — wires `otel.SetErrorHandler` so SDK-internal errors (export failures, dropped spans, invalid arguments) surface through your logger instead of the default logr sink. Wire to `xlog` for unified error surfacing.
- **`SpanContextInfo(ctx) (SpanInfo, bool)`** — pulls `TraceID`, `SpanID`, `Sampled` from ctx for log correlation. Use in any logger (xlog, slog, zap) without importing otel from the logger package. Enables trace ↔ log linking in Grafana Tempo + Loki.
- **Init idempotence** — calling `Init` twice returns an error. Use `Reset()` in tests to re-init. Prevents duplicate providers + resource leak.
- **`GO_LIB_OTEL_METRIC_INTERVAL`** env var support.

### Tests

- 5 → 8 tests. New coverage: Init idempotence + post-shutdown re-init, error handler callback wiring, `SpanContextInfo` valid + empty ctx, metric interval independent from batch timeout.


## v0.1.1 - 2026-08-12

### Docs

- Add `README.md` with quickstart, ENV table, options overview, end-to-end wire notes, sampling guidance, and bench excerpt.
- Add `example/` with a runnable service that emits one span + one counter.
- Add `bench_test.go` — 3 benches: `Tracer.Start/End` no-op (`NeverSample`), sampled (`AlwaysSample` + in-memory recorder), and resource build.


## v0.1.0 - 2026-08-12

Initial release.

- **`Init(ctx, opts...)`** wires tracer provider, meter provider, and global TextMapPropagator (TraceContext + Baggage by default). Returns `ShutdownFunc` that flushes both pipelines. Mirrors `xlog.Init` pattern.
- **`InitFromEnv(ctx, extra...)`** reads `GO_LIB_OTEL_*` + standard `OTEL_*` env vars; extra options override.
- **OTLP protocols** — gRPC (default) or HTTP/protobuf via `WithProtocol`.
- **Resource** — auto attributes: `service.name`, `service.version`, `deployment.environment`, process, host, telemetry SDK. Custom attrs via `WithResourceAttrs`.
- **Trace sampler** — parent-based ratio (default 1.0 always-on); override via `WithTraceSampler` or `GO_LIB_OTEL_TRACE_SAMPLE_RATIO` env.
- **Batch config** — `WithBatchTimeout`, `WithMaxExportBatchSize`, `WithMaxQueueSize`.
- **Opt-outs** — `WithoutTraces`, `WithoutMetrics` skip either pipeline entirely (useful for tests or edge builds).
- **Convenience** — `Tracer(name)` and `Meter(name)` return the global instances.
- **Tests** — 5 tests, race-clean: propagator install, custom propagator override, resource attribute wiring via in-memory span recorder, env override, exporter-unreachable smoke.
