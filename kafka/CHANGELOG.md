
## kafka/v0.3.0 - 2026-08-12

### Breaking

- **consumer:** `NewConsumer` reverts to single-topic signature (`topic string`) after the v0.2.0 multi-topic experiment. One consumer, one topic — cleaner API surface. Callers on v0.2.0 need to unwrap `[]string{topic}` back to `topic`.

### Features

- **producer:** `AsyncProducer[T]` — non-blocking `Publish` wrapping sarama's `AsyncProducer`. Enqueues to sarama's input channel and returns; delivery outcome (success or error) is delivered via `WithAsyncCallback[T]` on a drain goroutine. Trace context is injected on publish, same as sync `Producer[T]`. Close waits for in-flight messages to drain.
- **consumer:** `WithConcurrencyPerPartition(n)` — runs `n` workers per partition claim. Committer marks only the highest **contiguous** completed offset, so a hole from an in-flight message never advances the commit past it — at-least-once is preserved on rebalance. Default 1 (serial path). Trade-off: breaks per-key ordering; use only when handlers are commutative or ordering is enforced elsewhere.
- **producer:** `WithProducerLogger(Logger)` for consistent diagnostics.

### Tests

- Pool contiguous-commit test with variable per-offset latency (proves out-of-order processing still commits in order).
- Async producer callback fires on success and error (sarama `mocks.AsyncProducer`); Publish-after-Close returns an error.
- 15 tests total, race-clean.


## kafka/v0.2.0 - 2026-08-11

### Changes

- **kafka:** v0.2.0 — error strategies, DLQ, OTel tracing, idempotent producer


## kafka/v0.1.7 - 2026-08-11

### Changes

- **kafka:** update changelog for kafka/v0.1.7


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

