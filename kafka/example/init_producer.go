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

	// One producer: one topic + one message type. Key per message via WithKeyFunc (e.g. partition by ID).
	p, err := kafka.NewProducer[OrderCreated](
		brokers,
		topic,
		kafka.WithKeyFunc[OrderCreated](func(req OrderCreated) []byte {
			return []byte(req.ID)
		}),
		kafka.WithAcks(sarama.WaitForAll),
		kafka.WithIdempotent(),
		kafka.WithRetryMax(3),
		kafka.WithRetryBackoff(100*time.Millisecond),
		kafka.WithCompression(sarama.CompressionSnappy),
		kafka.WithTimeout(10*time.Second),
	)
	if err != nil {
		log.Fatalf("failed to create producer: %v", err)
	}
	defer func() {
		_ = p.Close()
	}()

	ctx := context.Background()

	// Publish one event (struct is encoded to JSON by the package)
	err = p.Publish(ctx, OrderCreated{ID: "1", Amount: 100})
	if err != nil {
		log.Fatalf("failed to publish: %v", err)
	}
	log.Printf("published OrderCreated id=1 amount=100")

	// PublishMany: send multiple events in one batch
	events := []OrderCreated{
		{ID: "2", Amount: 200},
		{ID: "3", Amount: 300},
	}
	err = p.PublishMany(ctx, events)
	if err != nil {
		log.Fatalf("failed to publish many: %v", err)
	}
	log.Printf("published %d events", len(events))
}
