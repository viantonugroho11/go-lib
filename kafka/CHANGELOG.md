
## kafka/v0.2.0 - 2026-08-11

### Breaking

- **consumer:** `NewConsumer` signature now takes `topics []string` (was `topic string`). Wrap old callers with `[]string{topic}`.
- **producer:** idempotent producer is now on by default (`acks=all`, `MaxOpenRequests=1`, `Retry.Max=5`). `WithIdempotent()` renamed to `WithIdempotent(enable bool)`; pass `false` to opt out.

### Bug Fixes

- **consumer:** fix silent-drop on handler error. Previously, returning `ProgressError` skipped `MarkMessage` for the failing offset but the next successful message committed a higher offset, permanently losing the failed one. Now controlled explicitly by `WithErrorStrategy`:
  - `ErrorSkip` (default) — log + advance (documented, no longer silent).
  - `ErrorBlock` — retry same message with exponential backoff until success or ctx cancel.
  - `ErrorDeadLetter` — publish to DLQ via `WithDeadLetter(fn)`, then advance; falls back to skip if DLQ publish fails.

### Features

- **consumer:** multi-topic support via `topics []string` argument.
- **consumer:** `WithLogger(Logger)` option — pluggable logger interface (`Errorf`/`Infof`); default writes to stderr. Wire `xlog` or any adapter.
- **consumer:** `WithErrorStrategy`, `WithDeadLetter`, `WithBlockOnError(BlockBackoff)` options.
- **producer / consumer:** OpenTelemetry trace context (`traceparent`, `tracestate`) is now injected on publish and extracted on consume, via the global `otel.TextMapPropagator`. No-op when no propagator is configured.
- **producer:** `WithEncoder[T](fn)` option for Avro/Protobuf/msgpack in place of the JSON default.

### Tests

- Added mock `ConsumerGroupSession` — proves offset-drop bug, verifies each error strategy, decode-failure path, header filtering, and OTel round-trip. 10 tests, race-clean.


## kafka/v0.1.6 - 2026-08-11

### Changes

- **deps:** bump vulnerable deps in config and kafka


## kafka/v0.1.5 - 2026-07-16

### Changes

- **kafka:** update changelog for kafka/v0.1.5
- update changelogs for kafka/v0.1.5, config/v0.1.2, xlog/v0.1.1
- **kafka:** clarify ctx not propagated in Publish/PublishMany
- **kafka:** replace deprecated BalanceStrategyRange with NewBalanceStrategyRange
- **kafka:** remove duplicate WithClientID/WithVersion options and orphan comment
- **kafka:** align EventProducer interface with Producer[T] signatures
- **kafka:** SetError now sets Status=ProgressError
- **kafka:** remove unused createConsumer helper
- **kafka:** prevent double option apply in NewConsumer
- **kafka:** fix deadlock in consumer.Close


## kafka/v0.1.4 - 2026-03-06

### Changes

- **kafka:** update changelog for kafka/v0.1.4
- Update module


## kafka/v0.1.3 - 2026-03-06

### Changes



## kafka/v0.1.2 - 2026-03-05

### Changes

- **kafka:** update changelog for kafka/v0.1.2
- update producer and add example


## kafka/v0.1.1 - 2026-03-05

### Changes

- **kafka:** update changelog for kafka/v0.1.1
- UPDATE implemtation
- update implemtation and remove unused func
- Update kafka implemtation
- update implementation kafka consumer
- update implementasi


## kafka/v0.1.0 - 2026-01-03

### Changes

- **kafka:** update changelog for kafka/v0.1.0
- **kafka:** fixed formating
- **docs:** update readme and update comment functional
- 

