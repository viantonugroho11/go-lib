package kafka

import (
	"context"
	"encoding/json"

	"github.com/IBM/sarama"
)

// Producer is a generic SyncProducer bound to one topic and one message type T.
// Values of type T are encoded to bytes (JSON by default) when publishing.
type Producer[T any] struct {
	sp           sarama.SyncProducer
	topicName    string
	key          []byte
	keyExtractor func(T) []byte // if set, key is computed per message; else key is used
	encode       func(T) ([]byte, error)
}

// NewProducer creates a producer for one topic and one message type T.
// The topic is fixed; use WithKey or WithKeyFunc to set the message key.
// Uses defaultProducerConfig(); options override defaults.
func NewProducer[T any](brokers []string, topic string, options ...ProducerOption) (*Producer[T], error) {
	build := &producerBuildConfig{cfg: defaultProducerConfig()}
	for _, opt := range options {
		opt.apply(build)
	}
	sp, err := sarama.NewSyncProducer(brokers, build.cfg)
	if err != nil {
		return nil, err
	}
	var keyExtractor func(T) []byte
	if build.keyExtractor != nil {
		keyExtractor, _ = build.keyExtractor.(func(T) []byte)
	}
	return &Producer[T]{
		sp:           sp,
		topicName:    topic,
		key:          build.key,
		keyExtractor: keyExtractor,
		encode:       func(v T) ([]byte, error) { return json.Marshal(v) },
	}, nil
}

// Close closes the underlying connection to Kafka.
func (p *Producer[T]) Close() error {
	return p.sp.Close()
}

// sendRaw sends a single raw message (key, value bytes) to the producer's topic.
func (p *Producer[T]) sendRaw(key []byte, value []byte, headers ...Header) (partition int32, offset int64, err error) {
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

// Publish encodes the value as JSON and sends one message. Key from WithKey or WithKeyFunc (per message).
func (p *Producer[T]) Publish(ctx context.Context, value T, headers ...Header) error {
	encoded, err := p.encode(value)
	if err != nil {
		return err
	}
	key := p.key
	if p.keyExtractor != nil {
		key = p.keyExtractor(value)
	}
	_, _, err = p.sendRaw(key, encoded, headers...)
	return err
}

// PublishMany encodes each value as JSON and sends messages in batch. Key from WithKey or WithKeyFunc (per message).
func (p *Producer[T]) PublishMany(ctx context.Context, values []T, headers ...Header) error {
	if len(values) == 0 {
		return nil
	}
	messages := make([]*sarama.ProducerMessage, 0, len(values))
	var saramaHeaders []sarama.RecordHeader
	if len(headers) > 0 {
		saramaHeaders = make([]sarama.RecordHeader, 0, len(headers))
		for _, h := range headers {
			saramaHeaders = append(saramaHeaders, sarama.RecordHeader{Key: []byte(h.Key), Value: h.Value})
		}
	}
	for _, v := range values {
		encoded, err := p.encode(v)
		if err != nil {
			return err
		}
		key := p.key
		if p.keyExtractor != nil {
			key = p.keyExtractor(v)
		}
		messages = append(messages, &sarama.ProducerMessage{
			Topic:   p.topicName,
			Key:     sarama.ByteEncoder(key),
			Value:   sarama.ByteEncoder(encoded),
			Headers: saramaHeaders,
		})
	}
	return p.sp.SendMessages(messages)
}
