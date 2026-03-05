// Package setup provides a single place to build Kafka consumers (e.g. for future config/tracing).
// Example: main.go uses CONSUMER_NAME=order_via_setup or repayment_via_setup to run via setup.
package setup

import (
	"context"

	"github.com/IBM/sarama"
	"kafka"
)

// SetupConsumer builds a Consumer from EventHandler[E].
func SetupConsumer[E any](ctx context.Context, brokers []string, groupID, topic string, handler kafka.EventHandler[E], opts ...kafka.ConsumerOption) (kafka.Consumer, error) {
	return kafka.NewConsumer[E](brokers, groupID, topic, handler, opts...)
}

// ConsumerOption re-exports kafka.ConsumerOption for options (TLS, offset, etc.).
type ConsumerOption = kafka.ConsumerOption

var (
	WithInitialOffset    = kafka.WithInitialOffset
	WithConsumerClientID = kafka.WithConsumerClientID
	WithStartFromOldest  = func() ConsumerOption { return kafka.WithInitialOffset(sarama.OffsetOldest) }
	WithStartFromNewest  = func() ConsumerOption { return kafka.WithInitialOffset(sarama.OffsetNewest) }
)
