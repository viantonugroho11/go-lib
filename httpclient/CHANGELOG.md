## httpclient/v0.1.1 - 2026-08-12

### Docs

- Add `README.md` with quickstart, verbs, retry rules, options, end-to-end correlation, and bench excerpt.
- Add `bench_test.go` — 3 benches: `Get` no-retry, `Get` with default + per-request headers, `Post` JSON body. Shared server + transport across benches to avoid port exhaustion.


## httpclient/v0.1.0 - 2026-07-16

### Changes

- feat: initial httpclient library with retry, timeout, default headers, and context correlation propagation
