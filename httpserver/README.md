## httpserver

Opinionated HTTP server on `chi/v5` with graceful shutdown, request ID, correlation propagation, panic recover, per-request timeout, structured logging hook, and health/ready endpoints.

### Quick Start

```go
srv := httpserver.New(
    httpserver.WithAddr(":8080"),
    httpserver.WithRequestTimeout(5*time.Second),
    httpserver.WithLogger(func(ctx context.Context, m, p string, s int, d time.Duration) {
        xlog.Logger().Info("http",
            xlog.Str("method", m), xlog.Str("path", p),
            xlog.Int("status", s), xlog.Dur("dur", d))
    }),
    httpserver.WithReadyCheck(db.PingContext),
)
srv.Router().Get("/users/{id}", getUser)

ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
_ = srv.Run(ctx)
```

### Built-in Middleware (applied in order)

1. **Request ID** — reads/generates `X-Request-ID`, echoes it, stores in ctx.
2. **Correlation headers** — extract any `WithCorrelationHeader` header into ctx.
3. **Panic recover** — 500 + log.
4. **Request timeout** — if `WithRequestTimeout` set.
5. **Logging hook** — fires after each request with method, path, status, duration.
6. **User middleware** — appended via `WithMiddleware`.

### Options

- **Networking**: `WithAddr`, `WithTimeouts(read, write, idle)`, `WithShutdownTimeout`.
- **Behavior**: `WithRequestTimeout`, `WithRequestID(header, ctxKey)`, `WithCorrelationHeader(header, ctxKey)`.
- **Logging**: `WithLogger(LoggerFunc)`.
- **Health**: `WithHealthPath` (`/healthz`), `WithReadyPath` (`/readyz`), `WithReadyCheck(fn)`.
- **Extension**: `WithMiddleware`, `WithoutRecover`.

### Health / Readiness

- `GET /healthz` — always 200 (liveness).
- `GET /readyz` — runs the `ReadyCheck` func; 200 if nil, 503 with body if error.

### End-to-End Tracing

Pair `WithCorrelationHeader` with `httpclient.WithCorrelationHeader` and boot `otel.Init` — trace context flows across the whole stack automatically. `RequestIDFromContext(ctx)` pulls the ID inside any handler.

### Router Access

`srv.Router()` returns the raw `chi.Router`. Full chi ecosystem (sub-router, middleware group, URL params) works. `srv.Handler()` returns the `http.Handler` for `httptest`.

### Benchmark

Apple M2 (arm64):

```
BenchmarkServer_HealthEndpoint         ~1.1 μs/op       ~1.8 KB/op   ~18 allocs/op
BenchmarkServer_UserHandler            ~1.2 μs/op       ~2.1 KB/op   ~19 allocs/op
BenchmarkRequestIDMiddleware_Generate  ~640 ns/op       ~976 B/op    ~11 allocs/op
BenchmarkRequestIDMiddleware_Passthru  ~340 ns/op       ~944 B/op    ~10 allocs/op
```

Router + built-in middleware chain adds ~1.3 μs to a no-op handler. Real handler cost dominates.

### Example

`example/` has a runnable server with a `GET /users/{id}` route, custom middleware, health check, and correlation-header wiring.
