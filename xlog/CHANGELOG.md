
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

