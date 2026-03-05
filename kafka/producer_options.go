package kafka

import (
	"time"

	"github.com/IBM/sarama"
)

// producerBuildConfig holds Sarama config and optional producer-level options (key or key extractor).
type producerBuildConfig struct {
	cfg          *sarama.Config
	key          []byte
	keyExtractor interface{} // func(T) []byte, same T as Producer[T]
}

// ProducerOption configures the producer (Sarama config and/or optional key).
type ProducerOption interface {
	apply(*producerBuildConfig)
}

type producerOptionFunc func(*producerBuildConfig)

func (f producerOptionFunc) apply(c *producerBuildConfig) { f(c) }

// defaultProducerConfig returns a Sarama producer config with best-practice defaults.
func defaultProducerConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_8_0_0
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 3
	cfg.Producer.Retry.Backoff = 100 * time.Millisecond
	cfg.Producer.Compression = sarama.CompressionSnappy
	cfg.Producer.Timeout = 10 * time.Second
	cfg.Producer.MaxMessageBytes = 1000000 // 1 MB
	return cfg
}

// WithKey sets a fixed message key for all Publish/PublishMany from this producer.
func WithKey(key []byte) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.key = key
	})
}

// WithKeyFunc sets a function to compute the message key from each value (e.g. partition by ID).
// T must match the type used in NewProducer[T]. If both WithKey and WithKeyFunc are set, WithKeyFunc wins.
func WithKeyFunc[T any](fn func(T) []byte) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.keyExtractor = fn
	})
}

// WithRetryBackoff sets the retry backoff duration.
func WithRetryBackoff(retryBackoff time.Duration) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.cfg.Producer.Retry.Backoff = retryBackoff
	})
}

// WithRetryMax sets the maximum number of retries.
func WithRetryMax(max int) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.cfg.Producer.Retry.Max = max
	})
}

// WithAcks sets the required acks.
func WithAcks(acks sarama.RequiredAcks) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.cfg.Producer.RequiredAcks = acks
	})
}

// WithIdempotent enables idempotent producer (exactly-once semantics; sets acks=all and MaxOpenRequests=1).
func WithIdempotent() ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.cfg.Producer.Idempotent = true
		c.cfg.Producer.RequiredAcks = sarama.WaitForAll
		c.cfg.Net.MaxOpenRequests = 1
	})
}

// WithCompression sets the producer compression codec.
func WithCompression(codec sarama.CompressionCodec) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.cfg.Producer.Compression = codec
	})
}

// WithTimeout sets the message publish timeout.
func WithTimeout(timeout time.Duration) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.cfg.Producer.Timeout = timeout
	})
}

// WithMaxMessageBytes sets the maximum message size.
func WithMaxMessageBytes(n int) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.cfg.Producer.MaxMessageBytes = n
	})
}

// WithReturnSuccesses sets return successes flag (SyncProducer requires true).
func WithReturnSuccesses(enable bool) ProducerOption {
	return producerOptionFunc(func(c *producerBuildConfig) {
		c.cfg.Producer.Return.Successes = enable
	})
}
