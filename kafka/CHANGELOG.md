
## kafka/v0.3.2 - 2026-08-12

### Performance

Benchmarked on Apple M2 (arm64). Per-message consumer hot path improved 15–26%; header-filter improved 68%.

- **consumer:** cache `tracingActive()` at `adaptEventHandler` construction time. Avoids the per-message `otel.GetTextMapPropagator().Fields()` call (composite propagator builds a map, ~24 B/1 alloc). If the user swaps the global propagator after starting a consumer, they must restart it.
- **consumer:** skip `headersFromMessage` allocation entirely when the message carries no headers, or when neither `WithHeaderKeys` was set nor tracing is active.
- **consumer:** `filterHeadersByKeys` now uses a linear scan for `len(keys) ≤ 4` — zero map allocation. Bench: 126 ns / 240 B → 40 ns / 80 B (-68% ns, -66% B).
- **producer / async producer:** cache tracing-active at construction time; `maybeInjectTrace` short-circuits when tracing is inactive.
- **consumer:** `WithDecoder[E]` — override the default JSON decoder for Avro, Protobuf, msgpack, or a faster JSON lib (goccy/go-json, jsoniter) without changing the handler.
- **docs:** `WithConcurrencyPerPartition` docstring now names the empirical break-even: handler cost > ~5 μs. Below that, the serial path is faster — pool channel + scheduling overhead dominates.


## kafka/v0.3.1 - 2026-08-11

### Changes

- (auto-tagged remotely; superseded by v0.3.2)


## kafka/v0.3.0 - 2026-08-12

### Changes

- **kafka:** v0.3.0 — async producer, worker pool per partition, single-topic API


## kafka/v0.2.1 - 2026-08-11

### Changes

- **kafka:** update changelog for kafka/v0.2.1


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

