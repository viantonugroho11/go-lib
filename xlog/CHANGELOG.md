
## xlog/v0.1.2 - 2026-08-12

### Docs

- Add `bench_test.go` — 4 benches: structured `Info` with 0 and 5 fields, sugared `Infow` with 5 fields, and ctx-aware `Info(ctx, ...)` with the context field extractor active.

Bench (Apple M2, arm64):

```
BenchmarkLogger_Info_NoFields       ~233 ns/op       0 B/op       0 allocs/op
BenchmarkLogger_Info_FiveFields     ~428 ns/op       320 B/op     1 alloc/op
BenchmarkSugar_Infow_FiveFields     ~596 ns/op       704 B/op     1 alloc/op
BenchmarkInfoCtx_WithExtractor      ~637 ns/op       1489 B/op    7 allocs/op
```


## xlog/v0.1.1 - 2026-07-16

### Bug Fixes

- **context_fields:** use `sync/atomic.Pointer` for `contextFieldExtractor` to prevent data race on concurrent `SetContextFieldExtractor` calls (e.g. parallel tests)

### Changes

- **context_fields:** move `Error` log helper here from `options.go` for consistency with other log-level helpers


## xlog/v0.1.0 - 2026-01-03

### Changes

- **config:** update example config
- **docs:** update readme and update comment functional
- **docs:** add readme
- update log
- 

