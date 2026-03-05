package kafka

import "context"

// Header represents a key-value pair found in Kafka message headers.
type Header struct {
	Key   string
	Value []byte
}

// GetHeader returns the first header value for the given key (case-sensitive), or nil if not found.
func GetHeader(headers []Header, key string) []byte {
	for _, h := range headers {
		if h.Key == key {
			return h.Value
		}
	}
	return nil
}

// GetHeaderString returns the first header value as string for the given key, or empty string if not found.
func GetHeaderString(headers []Header, key string) string {
	b := GetHeader(headers, key)
	if b == nil {
		return ""
	}
	return string(b)
}

// ProgressStatus is the result of processing one event.
type ProgressStatus int

const (
	ProgressSuccess ProgressStatus = iota
	ProgressSkip    // commit offset, do not retry
	ProgressDrop    // commit offset, do not retry
	ProgressError   // do not commit; message will be retried
)

func (s ProgressStatus) String() string {
	switch s {
	case ProgressSuccess:
		return "success"
	case ProgressSkip:
		return "skip"
	case ProgressDrop:
		return "drop"
	case ProgressError:
		return "error"
	default:
		return "unknown"
	}
}

// Progress holds the result of handling one event.
type Progress struct {
	Status ProgressStatus
	Result string
	Err    error
	Stage  string
}

// SetError sets Err and optionally Stage for observability.
func (p *Progress) SetError(err error) {
	p.Err = err
}

// EventHandler processes events of type E. Variadic headers may be ignored if not needed.
type EventHandler[E any] interface {
	Handle(ctx context.Context, evt E, headers ...Header) Progress
	Name() string
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
