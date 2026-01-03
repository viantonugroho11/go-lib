package main

import (
	"context"
	"log"

	"github.com/IBM/sarama"
	"kafka"
)

type OrderCreated struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

func main() {
	typed := func(ctx context.Context, msg *sarama.ConsumerMessage, evt OrderCreated) error {
		log.Printf("consumed topic=%s key=%s id=%s amount=%d", msg.Topic, string(msg.Key), evt.ID, evt.Amount)
		return nil
	}
	handler := kafka.AdaptTypedHandler[OrderCreated](
		typed,
		kafka.WithJSONDecoder[OrderCreated](),
	)

	// Set ENV KAFKA_* sesuai README sebelum menjalankan
	c, err := kafka.NewConsumerFromEnv("KAFKA_", "example-group", []string{"orders"}, handler)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	c.Start(context.Background())
	select {}
}


