package kafka

import (
	"context"
	"time"

	"github.com/IBM/sarama"
)

// Producer adalah pembungkus generic untuk SyncProducer Sarama
// dengan resolver topik bertipe T (misal string, enum, dsb).
type Producer[T any] struct {
	sp            sarama.SyncProducer
	topicName string
}

// ProducerOption memungkinkan kustomisasi konfigurasi producer sebelum dibuat.
type ProducerOption func(cfg *sarama.Config)

// NewProducer membuat SyncProducer baru dengan konfigurasi default + opsi.
func NewProducer[T any](brokers []string, topic string, options ...ProducerOption) (*Producer[T], error) {
	cfg := sarama.NewConfig()
	// Default yang aman untuk SyncProducer[T]
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

// Close menutup koneksi ke Kafka.
func (p *Producer[T]) Close() error {
	return p.sp.Close()
}

// SendMessage mengirim satu pesan ke Kafka.
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

// SendMessages mengirim banyak pesan ke Kafka.
func (p *Producer[T]) SendMessages(messages []sarama.ProducerMessage) (err error) {
	batch := make([]*sarama.ProducerMessage, len(messages))
	for i := range messages {
		batch[i] = &messages[i]
	}
	return p.sp.SendMessages(batch)
}

// Publish mengirim event dengan header opsional; kompatibel dengan interface EventProducer[E]
// ketika T adalah tipe topik (misal string) dan pemanggil melakukan serialisasi pada sisi mereka.
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

// WithRetryMax mengatur jumlah maksimum retry.
func WithRetryMax(max int) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Retry.Max = max
	}
}

// WithAcks mengatur required acks.
func WithAcks(acks sarama.RequiredAcks) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.RequiredAcks = acks
	}
}

// WithIdempotent mengaktifkan idempotent producer (secara implisit acks=all).
func WithIdempotent() ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Idempotent = true
		cfg.Producer.RequiredAcks = sarama.WaitForAll
	}
}

// WithCompression mengatur codec kompresi producer.
func WithCompression(codec sarama.CompressionCodec) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Compression = codec
	}
}

// WithTimeout mengatur timeout pengiriman message.
func WithTimeout(timeout time.Duration) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Timeout = timeout
	}
}

// WithMaxMessageBytes mengatur ukuran maksimum pesan.
func WithMaxMessageBytes(n int) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.MaxMessageBytes = n
	}
}

// WithReturnSuccesses mengatur flag return sukses (SyncProducer membutuhkan true).
func WithReturnSuccesses(enable bool) ProducerOption {
	return func(cfg *sarama.Config) {
		cfg.Producer.Return.Successes = enable
	}
}
