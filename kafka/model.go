package kafka

import "context"

// Header represents a key-value pair found in Kafka message headers.
type Header struct {
	Key   string
	Value []byte
}

type EventAndHeader[E any] struct {
	Headers []Header
	Event   E
}
// EventProducer is an interface for publishing events.
type EventProducer[E any] interface {
	// Publish publishes the given event with optional headers.
	// It returns an error if the publishing fails.
	Publish(ctx context.Context, evt E, headers ...Header) error
	// PublishMany publishes multiple events with optional headers.
	// It returns an error if the publishing fails.
	PublishMany(ctx context.Context, eahs ...EventAndHeader[E]) error
}
