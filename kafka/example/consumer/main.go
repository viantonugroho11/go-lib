package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/viantonugroho11/go-lib/kafka"
)

type OrderCreated struct {
	ID     string `json:"id"`
	Amount int    `json:"amount"`
}

type orderHandler struct{}

func (orderHandler) Name() string { return "order" }

func (orderHandler) Handle(ctx context.Context, evt OrderCreated, _ ...kafka.Header) kafka.Progress {
	log.Printf("consumed id=%s amount=%d", evt.ID, evt.Amount)
	return kafka.Progress{Status: kafka.ProgressSuccess}
}

func main() {
	brokersStr := os.Getenv("KAFKA_BROKERS")
	if brokersStr == "" {
		brokersStr = "localhost:9092"
	}
	brokers := strings.Split(brokersStr, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}
	c, err := kafka.NewConsumer[OrderCreated](brokers, "example-group", "orders", orderHandler{})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	c.Start(context.Background())
	select {}
}
