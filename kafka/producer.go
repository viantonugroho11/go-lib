package kafka

import (
	"context"
	"time"

	"github.com/IBM/sarama"
)

// Producer adalah pembungkus generic untuk SyncProducer Sarama
// with a topic typed as T (e.g., string, enum, etc.).
type Producer[T any] struct {
	sp        sarama.SyncProducer
	topicName string
}

// ProducerOption customizes sarama.Config before the producer is created.
type ProducerOption func(cfg *sarama.Config)

// NewProducer creates a new SyncProducer with sensible defaults plus options.
func NewProducer[T any](brokers []string, topic string, options ...ProducerOption) (*Producer[T], error) {
	cfg := sarama.NewConfig()
	// Safe defaults for SyncProducer[T]
	cfg.Producer.Return.Successes = true
	for _, option := range options {
		option(cfg)
	}
	sp, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, err
	}
	return &Producer[T]{sp: sp, topicName: topic}, nil
}

// Close closes the underlying connection to Kafka.
func (p *Producer[T]) Close() error {
	return p.sp.Close()
}

// SendMessage sends a single message to Kafka.
func (p *Producer[T]) SendMessage(topic T, key []byte, value []byte, headers ...Header) (partition int32, offset int64, err error) {
	var saramaHeaders []sarama.RecordHeader
	if len(headers) > 0 {
		saramaHeaders = make([]sarama.RecordHeader, 0, len(headers))
		for _, h := range headers {
			saramaHeaders = append(saramaHeaders, sarama.RecordHeader{Key: []byte(h.Key), Value: h.Value})
		}
	}
	return p.sp.SendMessage(&sarama.ProducerMessage{
		Topic:   p.topicName,
		Key:     sarama.ByteEncoder(key),
		Value:   sarama.ByteEncoder(value),
		Headers: saramaHeaders,
	})
}

// SendMessages sends a batch of messages to Kafka.
func (p *Producer[T]) SendMessages(messages []sarama.ProducerMessage) (err error) {
	batch := make([]*sarama.ProducerMessage, len(messages))
	for i := range messages {
		batch[i] = &messages[i]
	}
	return p.sp.SendMessages(batch)
}

// Publish sends an event with optional headers; compatible with EventProducer[E]
// when T is the topic type (e.g., string) and the caller handles serialization.
func (p *Producer[T]) Publish(ctx context.Context, topic T, key []byte, value []byte, headers ...Header) error {
	_, _, err := p.SendMessage(topic, key, value, headers...)
	return err
}

// with retry backoff
func WithRetryBackoff(retryBackoff time.Duration) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Retry.Backoff = retryBackoff
	}
}

// WithRetryMax sets the maximum number of retries.
func WithRetryMax(max int) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Retry.Max = max
	}
}

// WithAcks sets the required acks.
func WithAcks(acks sarama.RequiredAcks) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.RequiredAcks = acks
	}
}

// WithIdempotent enables idempotent producer (implicitly sets acks=all).
func WithIdempotent() ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Idempotent = true
		cfg.Producer.RequiredAcks = sarama.WaitForAll
	}
}

// WithCompression sets the producer compression codec.
func WithCompression(codec sarama.CompressionCodec) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Compression = codec
	}
}

// WithTimeout sets the message publish timeout.
func WithTimeout(timeout time.Duration) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Timeout = timeout
	}
}

// WithMaxMessageBytes sets the maximum message size.
func WithMaxMessageBytes(n int) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.MaxMessageBytes = n
	}
}

// WithReturnSuccesses sets return successes flag (SyncProducer requires true).
func WithReturnSuccesses(enable bool) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Return.Successes = enable
	}
}
