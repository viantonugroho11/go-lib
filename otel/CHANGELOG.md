# Changelog

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
