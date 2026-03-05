# kafka

Sarama-based Kafka library with a generic, interface-driven API: **Consumer** uses `EventHandler[E]`, **Producer** publishes to a single topic.

## Installation

```bash
go get github.com/IBM/sarama
```

---

## Consumer

Single constructor: **`NewConsumer[E]`** with **`EventHandler[E]`**. Events are decoded as JSON by default.

### EventHandler interface

Implement two methods:

- **`Handle(ctx context.Context, evt E, headers ...Header) Progress`** — process one event; return `Progress` to control commit/retry.
- **`Name() string`** — consumer name (for logging/metrics).

The `headers` parameter only contains keys registered via **`WithHeaderKeys`** when creating the consumer; if you don't use headers, you can write `_ ...kafka.Header`.

### Progress status

| Status           | Meaning                                                |
|------------------|--------------------------------------------------------|
| `ProgressSuccess` | Processing succeeded; offset is committed.             |
| `ProgressSkip`    | Skip (e.g. filter); offset committed, no retry.       |
| `ProgressDrop`    | Drop (invalid); offset committed, no retry.           |
| `ProgressError`   | Error; offset **not** committed, message will be retried. |

Use **`p.SetError(err)`** to set `Err` on Progress (for observability).

### Headers

- When **creating the consumer**: register header keys with **`WithHeaderKeys("correlation_id", "resource")`**. Only these keys are passed to `Handle`.
- Inside **Handle**: read values with **`kafka.GetHeaderString(headers, "correlation_id")`** or **`kafka.GetHeader(headers, "key")`** (returns `[]byte`).

If you don't use **WithHeaderKeys**, the `headers` slice in `Handle` is empty.

### Minimal example

```go
type OrderCreated struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

type orderHandler struct{}
func (orderHandler) Name() string { return "order" }
func (orderHandler) Handle(ctx context.Context, evt OrderCreated, _ ...kafka.Header) kafka.Progress {
	log.Printf("id=%s amount=%d", evt.ID, evt.Amount)
	return kafka.Progress{Status: kafka.ProgressSuccess}
}

// Usage
c, err := kafka.NewConsumer[OrderCreated](
	[]string{"localhost:9092"},
	"example-group",
	"orders",
	orderHandler{},
	kafka.WithInitialOffset(sarama.OffsetOldest),
)
defer c.Close()
c.Start(ctx)
```

### Example with headers and validation (Skip/Drop/Error)

```go
func (h RepaymentHandler) Handle(ctx context.Context, evt RepaymentEvent, headers ...kafka.Header) kafka.Progress {
	if evt.ResourceID == "" {
		return kafka.Progress{Status: kafka.ProgressDrop, Result: "resource_id empty"}
	}
	correlationID := kafka.GetHeaderString(headers, "correlation_id")
	log.Printf("resource_id=%s correlation_id=%s", evt.ResourceID, correlationID)
	return kafka.Progress{Status: kafka.ProgressSuccess, Result: "ok"}
}

// When creating the consumer, register headers that can be read:
c, err := kafka.NewConsumer[RepaymentEvent](brokers, groupID, topic, RepaymentHandler{},
	kafka.WithHeaderKeys("correlation_id", "resource"),
)
```

### Consumer options

| Option | Description |
|--------|-------------|
| `WithHeaderKeys(keys ...string)` | Only these keys are passed to `Handle`; if not set, the handler receives no headers. |
| `WithConsumerClientID(id string)` | Sarama client ID. |
| `WithInitialOffset(offset)` | `sarama.OffsetNewest` or `sarama.OffsetOldest`. |
| `WithConsumerVersion(version)` | Kafka version. |
| `WithRebalanceStrategy(s)` | `BalanceStrategyRange`, `RoundRobin`, `Sticky`. |
| `WithTLSEnable(skipVerify bool)` | Enable TLS. |
| `WithSASLPlain(user, pass)` | SASL PLAIN authentication. |
| `WithMetricRegistry(registry)` | Metric registry (e.g. `go-metrics` / Prometheus). |

Brokers and other config are set in code; read from env (e.g. `os.Getenv("KAFKA_BROKERS")`) in your app if needed.

---

## Producer

One producer is bound to **one topic** and **one message type T** (your struct). Create with **`NewProducer[T](brokers, topic, options...)`**. The package encodes T to bytes (JSON by default); you pass structs, not `[]byte`. Message key: **`WithKey(key []byte)`** for a fixed key, or **`WithKeyFunc[T](fn func(T) []byte)`** to compute the key per message (e.g. partition by ID).

- **`Publish(ctx, value T, headers ...Header) error`** — encode one value and send.
- **`PublishMany(ctx, values []T, headers ...Header) error`** — encode and send a batch.

```go
type OrderCreated struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

p, err := kafka.NewProducer[OrderCreated](
	[]string{"localhost:9092"},
	"orders",
	kafka.WithKey([]byte("partition-key")),
	kafka.WithAcks(sarama.WaitForAll),
	kafka.WithIdempotent(),
	kafka.WithRetryMax(5),
)
defer p.Close()

// Send struct; package encodes to JSON
err = p.Publish(ctx, OrderCreated{ID: "1", Amount: 100})

// Batch
err = p.PublishMany(ctx, []OrderCreated{
	{ID: "2", Amount: 200},
	{ID: "3", Amount: 300},
})
```

### Producer options

| Option | Description |
|--------|-------------|
| `WithKey(key []byte)` | Fixed message key for all Publish/PublishMany. |
| `WithKeyFunc[T](fn func(T) []byte)` | Key per message from value (e.g. `[]byte(req.ID)`); T must match `NewProducer[T]`. |
| `WithRetryBackoff(d time.Duration)` | Retry backoff duration. |
| `WithRetryMax(n int)` | Maximum number of retries. |
| `WithAcks(acks sarama.RequiredAcks)` | Required acks. |
| `WithIdempotent()` | Enable idempotent producer (sets acks=all). |
| `WithCompression(codec)` | Compression codec. |
| `WithTimeout(d time.Duration)` | Message publish timeout. |
| `WithMaxMessageBytes(n int)` | Max message size. |
| `WithReturnSuccesses(enable bool)` | Return successes (SyncProducer typically needs true). |

---

## Example: example/producer

Minimal producer: one topic, one message type. Uses **Publish** (single event) and **PublishMany** (batch). Optional key via **WithKey**.

**Structure:**

```
kafka/example/producer/
  main.go
```

**`main.go` (summary):**

- Event type: `OrderCreated` with `id`, `amount` (JSON).
- **NewProducer[OrderCreated]** with brokers, topic, and options: `WithKey`, `WithAcks`, `WithIdempotent`, `WithRetryMax`, etc.
- **Publish(ctx, value)** — send one struct; package encodes to JSON.
- **PublishMany(ctx, values)** — send a slice of structs in one batch.

**Run:**

```bash
cd kafka
# Optional: KAFKA_BROKERS=localhost:9092 KAFKA_TOPIC=orders
go run ./example/producer/
```

Ensure Kafka is running and the topic exists.

---

## Example: example/consumer

Minimal example: one consumer for topic `orders` with event type `OrderCreated`.

**Structure:**

```
kafka/example/consumer/
  main.go
```

**`main.go` (summary):**

- Event type: `OrderCreated` with `id`, `amount` (JSON).
- Handler: `orderHandler` with `Name()` and `Handle(ctx, evt OrderCreated, _ ...kafka.Header) Progress` that logs and returns `ProgressSuccess`.
- `main`: read brokers from env `KAFKA_BROKERS` (default `localhost:9092`), call `kafka.NewConsumer[OrderCreated](brokers, "example-group", "orders", orderHandler{})`, then `Start(ctx)`.

**Run:**

```bash
cd kafka
# Optional: export KAFKA_BROKERS=localhost:9092
go run ./example/consumer/
```

Ensure Kafka is running and topic `orders` exists; create it with a producer or Kafka tools if needed.

---

## Example: exampl2 (multiple consumers + setup)

Example with **multiple consumer types** (order, repayment) and a **setup** layer as a single initialization point.

**Structure:**

```
kafka/exampl2/
  main.go           # Entry: choose consumer via CONSUMER_NAME, then run
  handlers/
    handle.go       # OrderEvent, RepaymentEvent, OrderHandler, RepaymentHandler, NewOrderConsumer, NewRepaymentConsumer
  setup/
    setup.go        # SetupConsumer: wrapper around kafka.NewConsumer[E]
```

**handlers/handle.go**

- **OrderEvent** / **RepaymentEvent**: event structs (JSON).
- **OrderHandler**: `EventHandler[OrderEvent]`; no headers (`_ ...kafka.Header`); simple validation (e.g. empty ID → `ProgressDrop`).
- **RepaymentHandler**: `EventHandler[RepaymentEvent]`; uses header `correlation_id` via `kafka.GetHeaderString(headers, "correlation_id")`.
- **NewOrderConsumer** / **NewRepaymentConsumer**: call `kafka.NewConsumer[OrderEvent]` / `NewConsumer[RepaymentEvent]` with handler and options (WithInitialOffset, WithHeaderKeys).

**main.go**

- Read env: `KAFKA_BROKERS`, `CONSUMER_NAME`, `KAFKA_GROUP_ID`, `KAFKA_TOPIC` (defaults: order_consumer, example-group, orders).
- Switch on `CONSUMER_NAME`:
  - `order_consumer` → `handlers.NewOrderConsumer(..., kafka.WithInitialOffset(sarama.OffsetOldest))`
  - `repayment_consumer` → `handlers.NewRepaymentConsumer(..., kafka.WithHeaderKeys("correlation_id", "resource"))`
  - `order_via_setup` → `setup.SetupConsumer[handlers.OrderEvent](..., handlers.OrderHandler{}, ...)`
  - `repayment_via_setup` → `setup.SetupConsumer[handlers.RepaymentEvent](..., handlers.RepaymentHandler{}, ...)`
- Graceful shutdown: `signal.NotifyContext`, `c.Start(ctx)`, `<-ctx.Done()`, `c.Close()`.

**setup/setup.go**

- **SetupConsumer[E]** simply calls `kafka.NewConsumer[E](brokers, groupID, topic, handler, opts...)`.
- Re-exports options (WithInitialOffset, WithStartFromOldest, etc.) so main can use them from the `setup` package if desired.

**Run:**

```bash
cd kafka

# Order consumer (no headers)
CONSUMER_NAME=order_consumer go run ./exampl2/

# Repayment consumer (with headers correlation_id, resource)
CONSUMER_NAME=repayment_consumer KAFKA_TOPIC=repayments go run ./exampl2/

# Same but via setup
CONSUMER_NAME=order_via_setup go run ./exampl2/
CONSUMER_NAME=repayment_via_setup KAFKA_TOPIC=repayments go run ./exampl2/
```

Env used: `KAFKA_BROKERS` (default localhost:9092), `CONSUMER_NAME`, `KAFKA_GROUP_ID`, `KAFKA_TOPIC`.

---

## Package layout

| File | Contents |
|------|----------|
| **consumer.go** | `Consumer` interface (Start, Close), `NewConsumer[E]`, consumer group implementation. |
| **options.go** | `ConsumerOption`, `WithHeaderKeys`, `WithInitialOffset`, TLS, SASL, MetricRegistry, etc. |
| **handler.go** | Internal adapter: EventHandler[E] → JSON decode + header filter → Sarama handler. |
| **model.go** | `Header`, `Progress`, `ProgressStatus`, `EventHandler[E]`, `GetHeader` / `GetHeaderString`. |
| **producer.go** | `Producer[T]`, `NewProducer`, `Publish`, `PublishMany`, `Close`. |
| **producer_options.go** | `ProducerOption`, `defaultProducerConfig`, `WithKey`, `WithKeyFunc`, `WithRetryBackoff`, `WithRetryMax`, `WithAcks`, `WithIdempotent`, etc. |
| **kafka.go** | Package documentation and short example. |
