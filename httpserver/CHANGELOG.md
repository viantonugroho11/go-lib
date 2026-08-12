# Changelog

## v0.1.2 - 2026-08-12

### Docs

- Add `README.md` with quickstart, middleware order, options, health/ready notes, tracing wire, and bench excerpt.
- Add `example/` with a chi handler, correlation header, health check, and panic recover demo.
- Add `bench_test.go` — 4 benches: `/healthz` end-to-end, user handler, request-ID middleware in both passthru and generate modes.


## v0.1.1 - 2026-08-11

### Security

- **deps:** bump `github.com/go-chi/chi/v5` to v5.3.1 (fixes CVE < 5.2.2)


## v0.1.0

Initial release.

- `New(opts...)` — chi-based server with graceful `Run(ctx)` shutdown.
- Built-in middleware: request ID, correlation headers, panic recover, request timeout, logging hook.
- Health (`/healthz`) and readiness (`/readyz`) endpoints with pluggable `ReadyCheck`.
- Options: `WithAddr`, `WithTimeouts`, `WithShutdownTimeout`, `WithRequestTimeout`, `WithRequestID`, `WithCorrelationHeader`, `WithLogger`, `WithHealthPath`, `WithReadyPath`, `WithReadyCheck`, `WithoutRecover`, `WithMiddleware`.
- `RequestIDFromContext` helper.
