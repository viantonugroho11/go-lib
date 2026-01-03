# kafka (Sarama-based Producer & Consumer)

Small library to bootstrap Kafka using Sarama with a generic, ergonomic, and production-friendly Producer and Consumer. Supports configuration via code options and environment variables (with a configurable prefix).

## Installation

Add dependencies:

```bash
go get github.com/IBM/sarama@v1.45.1
go get github.com/xdg-go/scram@v1.2.0
```

## Quick Usage

### 1) Configure via options (code)

```go
import (
	"time"
	"kafka"
	"github.com/IBM/sarama"
)

// Generic topic resolver; here T = string
topicResolver := func(topic string) string { return topic }

producer, err := kafka.NewProducer[string](
	[]string{"localhost:9092"},
	topicResolver,
	kafka.WithClientID("my-app"),
	kafka.WithAcks(sarama.WaitForAll),
	kafka.WithIdempotent(),
	kafka.WithCompression(sarama.CompressionSnappy),
	kafka.WithRetryMax(5),
	kafka.WithRetryBackoff(100*time.Millisecond),
	kafka.WithTimeout(10*time.Second),
	kafka.WithVersion(sarama.V2_6_0_0),
)
if err != nil {
	panic(err)
}
defer producer.Close()

_, _, err = producer.SendMessage("my-topic", []byte("my-key"), []byte("hello world"))
if err != nil {
	panic(err)
}
```

### 2) Configure via Environment Variables

Use a prefix (e.g., `KAFKA_`) to keep `.env` tidy:

```
KAFKA_BROKERS=localhost:9092
KAFKA_CLIENT_ID=my-app
KAFKA_VERSION=3.6.0
KAFKA_REQUIRED_ACKS=all
KAFKA_IDEMPOTENT=true
KAFKA_COMPRESSION=snappy
KAFKA_MAX_RETRY=5
KAFKA_RETRY_BACKOFF_MS=100
KAFKA_TIMEOUT_MS=10000
# Optional TLS/SASL:
# KAFKA_TLS_ENABLE=true
# KAFKA_TLS_INSECURE_SKIP_VERIFY=true
# KAFKA_SASL_ENABLE=true
# KAFKA_SASL_MECHANISM=SCRAM-SHA-512
# KAFKA_SASL_USERNAME=user
# KAFKA_SASL_PASSWORD=pass
```

Code:

```go
import "kafka"

topicResolver := func(topic string) string { return topic }

producer, err := kafka.NewProducerFromEnv[string]("KAFKA_", topicResolver)
if err != nil {
	panic(err)
}
defer producer.Close()

_, _, err = producer.SendMessage("my-topic", []byte("key"), []byte("value"))
if err != nil {
	panic(err)
}
```

## Notes
- Default `ClientID` is `go-lib-kafka` if not set.
- Default `RequiredAcks` is `all`.
- If `Idempotent = true`, acks will be forced to `all`.
- Supports SASL `PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512` and optional TLS.

## Consumer

### Init via code

```go
import (
	"context"
	"kafka"
	"github.com/IBM/sarama"
)

handler := func(ctx context.Context, msg *sarama.ConsumerMessage) error {
	// manually process the message
	return nil
}

c, err := kafka.NewConsumer(
	[]string{"localhost:9092"},
	"my-group",
	[]string{"topic-a","topic-b"},
	handler,
	kafka.WithConsumerClientID("my-consumer"),
	kafka.WithInitialOffset(sarama.OffsetOldest),
	kafka.WithRebalanceStrategy(sarama.BalanceStrategyRoundRobin),
)
if err != nil { panic(err) }
defer c.Close()
c.Start(context.Background())
```

### Init via ENV

```
KAFKA_BROKERS=localhost:9092
KAFKA_CLIENT_ID=my-consumer
KAFKA_VERSION=2.8.0
KAFKA_OFFSET_INITIAL=oldest # atau newest
KAFKA_REBALANCE_STRATEGY=range # or round_robin, sticky
# Optional TLS/SASL (PLAIN):
# KAFKA_TLS_ENABLE=true
# KAFKA_TLS_INSECURE_SKIP_VERIFY=true
# KAFKA_SASL_ENABLE=true
# KAFKA_SASL_MECHANISM=PLAIN
# KAFKA_SASL_USERNAME=user
# KAFKA_SASL_PASSWORD=pass
```

```go
import (
	"context"
	"kafka"
)

handler := func(ctx context.Context, msg *sarama.ConsumerMessage) error {
	return nil
}

c, err := kafka.NewConsumerFromEnv("KAFKA_", "my-group", []string{"topic-a"}, handler)
if err != nil { panic(err) }
defer c.Close()
c.Start(context.Background())
```

### Typed Handler (auto-unmarshal JSON)

```go
type OrderCreated struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

typed := func(ctx context.Context, msg *sarama.ConsumerMessage, evt OrderCreated) error {
	// evt is already unmarshaled
	return nil
}

handler := kafka.AdaptTypedHandler[OrderCreated](
	typed,
	kafka.WithJSONDecoder[OrderCreated](),
	// kafka.WithNewEvent(func() OrderCreated { return OrderCreated{Amount: 1} }),
)

c, err := kafka.NewConsumerFromEnv("KAFKA_", "my-group", []string{"orders"}, handler)
if err != nil { panic(err) }
defer c.Close()
c.Start(context.Background())
```

