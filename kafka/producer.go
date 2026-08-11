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
// Uses defaultProducerConfig() (idempotent + acks=all); options override defaults.
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
	var encode func(T) ([]byte, error)
	if build.encoder != nil {
		if fn, ok := build.encoder.(func(T) ([]byte, error)); ok {
			encode = fn
		}
	}
	if encode == nil {
		encode = func(v T) ([]byte, error) { return json.Marshal(v) }
	}
	return &Producer[T]{
		sp:           sp,
		topicName:    topic,
		key:          build.key,
		keyExtractor: keyExtractor,
		encode:       encode,
	}, nil
}

// Close closes the underlying connection to Kafka.
func (p *Producer[T]) Close() error {
	return p.sp.Close()
}

// Publish encodes the value and sends one message. Key from WithKey or WithKeyFunc (per message).
// Injects the ctx trace context (OTel global propagator) into the message headers.
// ctx cancellation is not propagated into sarama's SyncProducer wait — Timeout config bounds it.
func (p *Producer[T]) Publish(ctx context.Context, value T, headers ...Header) error {
	encoded, err := p.encode(value)
	if err != nil {
		return err
	}
	key := p.key
	if p.keyExtractor != nil {
		key = p.keyExtractor(value)
	}
	msg := &sarama.ProducerMessage{
		Topic:   p.topicName,
		Key:     sarama.ByteEncoder(key),
		Value:   sarama.ByteEncoder(encoded),
		Headers: toRecordHeaders(injectTrace(ctx, cloneHeaders(headers))),
	}
	_, _, err = p.sp.SendMessage(msg)
	return err
}

// PublishMany encodes each value and sends messages in batch. Trace context is injected once
// (shared across the batch); each message gets a copy of the merged header set.
func (p *Producer[T]) PublishMany(ctx context.Context, values []T, headers ...Header) error {
	if len(values) == 0 {
		return nil
	}
	sharedHeaders := injectTrace(ctx, cloneHeaders(headers))
	recordHeaders := toRecordHeaders(sharedHeaders)
	messages := make([]*sarama.ProducerMessage, 0, len(values))
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
			Headers: recordHeaders,
		})
	}
	return p.sp.SendMessages(messages)
}

func cloneHeaders(in []Header) []Header {
	if len(in) == 0 {
		return nil
	}
	out := make([]Header, len(in))
	copy(out, in)
	return out
}

func toRecordHeaders(in []Header) []sarama.RecordHeader {
	if len(in) == 0 {
		return nil
	}
	out := make([]sarama.RecordHeader, 0, len(in))
	for _, h := range in {
		out = append(out, sarama.RecordHeader{Key: []byte(h.Key), Value: h.Value})
	}
	return out
}
