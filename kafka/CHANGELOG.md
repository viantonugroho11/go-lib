
## kafka/v0.1.5 - 2026-07-16

### Bug Fixes

- **consumer:** fix deadlock in `Close()` — `group.Close()` must precede `wg.Wait()` so error goroutine can exit
- **consumer:** prevent double option apply in `NewConsumer` — options were applied twice causing sarama config mutations to fire twice
- **consumer:** remove unused `createConsumer` helper after refactor
- **model:** `SetError` now sets `Status=ProgressError` — previously only stored err, causing silent offset commit
- **model:** align `EventProducer` interface `PublishMany` signature with `Producer[T]` implementation

### Changes

- **options:** remove duplicate `WithClientID`/`WithVersion` (identical to `WithConsumerClientID`/`WithConsumerVersion`)
- **options:** replace deprecated `sarama.BalanceStrategyRange` with `NewBalanceStrategyRange()` to avoid data race
- **options:** remove orphan comment after `WithReturnErrors`
- **producer:** document that `ctx` is not propagated in `Publish`/`PublishMany` (sarama limitation)


## kafka/v0.1.4 - 2026-03-06

### Changes

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

