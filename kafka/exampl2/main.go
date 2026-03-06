package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/IBM/sarama"
	"github.com/viantonugroho11/go-lib/kafka"
	"github.com/viantonugroho11/go-lib/kafka/exampl2/handlers"
	"github.com/viantonugroho11/go-lib/kafka/exampl2/setup"
)

func main() {
	brokersStr := os.Getenv("KAFKA_BROKERS")
	if brokersStr == "" {
		brokersStr = "localhost:9092"
	}
	brokers := strings.Split(brokersStr, ",")
	for i := range brokers {
		brokers[i] = strings.TrimSpace(brokers[i])
	}

	consumerName := os.Getenv("CONSUMER_NAME")
	if consumerName == "" {
		consumerName = "order_consumer"
	}

	groupID := os.Getenv("KAFKA_GROUP_ID")
	if groupID == "" {
		groupID = "example-group"
	}

	topic := os.Getenv("KAFKA_TOPIC")
	if topic == "" {
		topic = "orders"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var cnsmr kafka.Consumer
	var err error

	switch consumerName {
	case "order_consumer":
		cnsmr, err = handlers.NewOrderConsumer(brokers, groupID, topic, kafka.WithInitialOffset(sarama.OffsetOldest))
	case "repayment_consumer":
		cnsmr, err = handlers.NewRepaymentConsumer(brokers, groupID, topic, kafka.WithHeaderKeys("correlation_id", "resource"))
	case "order_via_setup":
		// Contoh pakai setup: satu titik inisialisasi (nanti bisa dipakai untuk config/tracing).
		cnsmr, err = setup.SetupConsumer[handlers.OrderEvent](ctx, brokers, groupID, topic, handlers.OrderHandler{}, kafka.WithInitialOffset(sarama.OffsetOldest))
	case "repayment_via_setup":
		cnsmr, err = setup.SetupConsumer[handlers.RepaymentEvent](ctx, brokers, groupID, topic, handlers.RepaymentHandler{}, kafka.WithHeaderKeys("correlation_id", "resource"))
	default:
		log.Printf("consumer %q tidak dikenali. Gunakan: order_consumer, repayment_consumer, order_via_setup, repayment_via_setup", consumerName)
		os.Exit(1)
	}

	if err != nil {
		log.Fatalf("gagal membuat consumer: %v", err)
	}

	go cnsmr.Start(ctx)
	<-ctx.Done()
	if err := cnsmr.Close(); err != nil {
		log.Printf("close consumer: %v", err)
	}
}
