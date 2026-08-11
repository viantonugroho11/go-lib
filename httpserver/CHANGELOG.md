# Changelog

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
