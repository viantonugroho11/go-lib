// Example producer: one topic, one message type. Uses Publish and PublishMany with optional WithKey.
package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/viantonugroho11/go-lib/kafka"

	"github.com/IBM/sarama"
)

// OrderCreated is the event type for topic "orders" (one producer = one topic + one struct type).
type OrderCreated struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

func main() {
	brokersStr := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
	var brokers []string
	if brokersStr == "" {
		brokers = []string{"localhost:9092"}
	} else {
		for _, b := range strings.Split(brokersStr, ",") {
			if bb := strings.TrimSpace(b); bb != "" {
				brokers = append(brokers, bb)
			}
		}
	}

	topic := strings.TrimSpace(os.Getenv("KAFKA_TOPIC"))
	if topic == "" {
		topic = "orders"
	}

	// NewProducer[T]: one topic + one message type. Optional key via WithKey (applied to all messages).
	p, err := kafka.NewProducer[OrderCreated](
		brokers,
		topic,
		kafka.WithKey([]byte("orders-partition-key")),
		kafka.WithAcks(sarama.WaitForAll),
		kafka.WithIdempotent(),
		kafka.WithRetryMax(3),
		kafka.WithRetryBackoff(100*time.Millisecond),
		kafka.WithCompression(sarama.CompressionSnappy),
		kafka.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatalf("create producer: %v", err)
	}
	defer func() {
		_ = p.Close()
	}()

	ctx := context.Background()

	// Publish one event (struct is JSON-encoded by the package)
	if err := p.Publish(ctx, OrderCreated{ID: "1", Amount: 100}); err != nil {
		log.Fatalf("publish: %v", err)
	}
	log.Printf("published OrderCreated id=1 amount=100")

	// PublishMany: batch of events
	events := []OrderCreated{
		{ID: "2", Amount: 200},
		{ID: "3", Amount: 300},
	}
	if err := p.PublishMany(ctx, events); err != nil {
		log.Fatalf("publish many: %v", err)
	}
	log.Printf("published %d events", len(events))
}
