package main

import (
	"log"
	"os"
	"strings"
	"time"

	"kafka"

	"github.com/IBM/sarama"
)

func main() {
	// Read brokers from ENV (fallback: localhost:9092)
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

	// Read topic from ENV (fallback: example-topic)
	topic := strings.TrimSpace(os.Getenv("KAFKA_TOPIC"))
	if topic == "" {
		topic = "example-topic"
	}

	// Initialize producer with typical options
	p, err := kafka.NewProducer[string](
		brokers,
		topic,
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

	// Send a single message
	partition, offset, err := p.SendMessage(topic, []byte("key"), []byte("hello from example"))
	if err != nil {
		log.Fatalf("failed to send message: %v", err)
	}
	log.Printf("message sent to partition=%d, offset=%d", partition, offset)
}
