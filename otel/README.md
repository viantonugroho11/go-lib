## otel

OpenTelemetry bootstrap: tracer + meter + global TextMapPropagator behind one `Init` call. Mirrors the `xlog.Init` lifecycle.

### Quick Start

```go
shutdown, err := otel.Init(context.Background(),
    otel.WithServiceName("payments"),
    otel.WithServiceVersion("1.4.2"),
    otel.WithEnvironment("prod"),
    otel.WithEndpoint("otel-collector.observability:4317"),
)
if err != nil { log.Fatal(err) }
defer shutdown(context.Background())

tracer := otel.Tracer("payments/service")
ctx, span := tracer.Start(ctx, "ChargeCard")
defer span.End()
```

### ENV Bootstrap

```go
shutdown, _ := otel.InitFromEnv(ctx)
defer shutdown(ctx)
```

Recognized env vars (`GO_LIB_OTEL_*` overrides standard `OTEL_*`):

| Var | Default |
|-----|---------|
| `GO_LIB_OTEL_SERVICE_NAME` / `OTEL_SERVICE_NAME` | `unknown-service` |
| `GO_LIB_OTEL_SERVICE_VERSION` | — |
| `GO_LIB_OTEL_ENVIRONMENT` | — |
| `GO_LIB_OTEL_ENDPOINT` / `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` |
| `GO_LIB_OTEL_PROTOCOL` / `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` |
| `GO_LIB_OTEL_INSECURE` | `true` |
| `GO_LIB_OTEL_TRACE_SAMPLE_RATIO` | `1.0` |
| `GO_LIB_OTEL_DISABLE_TRACES` | `false` |
| `GO_LIB_OTEL_DISABLE_METRICS` | `false` |

### Options

- **Identity**: `WithServiceName`, `WithServiceVersion`, `WithEnvironment`, `WithResourceAttrs`.
- **Transport**: `WithProtocol(ProtocolGRPC|ProtocolHTTP)`, `WithEndpoint`, `WithHeaders`, `WithInsecure`, `WithTLSConfig(*tls.Config)`.
- **Sampling**: `WithTraceSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.05)))`.
- **Batching**: `WithBatchTimeout`, `WithMetricInterval`, `WithMaxExportBatchSize`, `WithMaxQueueSize`.
- **Opt-out**: `WithoutTraces`, `WithoutMetrics`, `WithoutLogs`.
- **Propagators**: `WithPropagators` (default `TraceContext + Baggage`); helpers `PropagatorB3()`, `PropagatorJaeger()`.
- **Metrics variants**: `WithRuntimeMetrics()` (Go runtime auto-instrumentation), `WithPrometheusExporter(mux, path)` (scrape endpoint alongside OTLP).
- **Dev / debug**: `WithStdoutExporter()` (traces + metrics + logs → stdout), `WithSpanProcessor(sp)` (append custom span processor), `WithErrorHandler(fn)` (SDK-internal error callback).
- **Log correlation helper**: `SpanContextInfo(ctx) (SpanInfo, bool)` — returns `TraceID`, `SpanID`, `Sampled`. Wire into your logger without importing otel from the logger package.

### End-to-End Wire

Once `Init` runs, the global propagator is picked up by:
- `kafka` producer + consumer (injects `traceparent` on publish, extracts on consume)
- `httpclient` via `WithCorrelationHeader`
- `httpserver` via `WithCorrelationHeader`

Zero per-call wiring in the handler.

### Benchmark

Apple M2 (arm64):

```
BenchmarkTracer_StartEnd_NoOp       ~145 ns/op       ~144 B/op    ~2 allocs/op
BenchmarkTracer_StartEnd_Sampled    ~567 ns/op       ~1.1 KB/op   ~3 allocs/op
BenchmarkBuildResource              ~14 μs/op        ~15 KB/op    ~88 allocs/op
```

Init is boot-time only; hot path is `Tracer.Start / End`. Sampled span (~570 ns) fine for anything above per-loop iteration.

### Sampling notes

- Production default at high traffic: `TraceIDRatioBased(0.05)` (5%).
- Use `ParentBased` so downstream services honor the parent's sample decision — otherwise a sampled ingress span may drop its outbound child spans.

### Example

See `example/` for a runnable service that emits one span + one metric to `stdout` via the SDK's stdout exporter (no collector required).
