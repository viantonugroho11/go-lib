package kafka

import "context"

// Header represents a key-value pair found in Kafka message headers.
type Header struct {
	Key   string
	Value []byte
}

// HeaderGet mengembalikan value header pertama dengan key yang cocok (case-sensitive). Nilai nil jika tidak ada.
func HeaderGet(headers []Header, key string) []byte {
	for _, h := range headers {
		if h.Key == key {
			return h.Value
		}
	}
	return nil
}

// HeaderGetString sama seperti HeaderGet tetapi mengembalikan string. Kosong jika tidak ada.
func HeaderGetString(headers []Header, key string) string {
	b := HeaderGet(headers, key)
	if b == nil {
		return ""
	}
	return string(b)
}

// ProgressStatus menyatakan hasil pemrosesan satu event (clean architecture: domain).
type ProgressStatus int

const (
	ProgressSuccess ProgressStatus = iota
	ProgressSkip    // skip tanpa retry, offset di-commit
	ProgressDrop    // drop tanpa retry, offset di-commit
	ProgressError   // error, offset tidak di-commit (akan retry)
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

// Progress hasil pemrosesan satu event oleh EventHandler.
type Progress struct {
	Status ProgressStatus
	Result string
	Err    error
	Stage  string
}

// SetError mengisi Err dan optional Stage (untuk observability).
func (p *Progress) SetError(err error) {
	p.Err = err
}

// EventHandler adalah interface untuk handler event (clean architecture: domain).
// Implementasi di layer application/use-case; package kafka hanya bergantung pada interface ini.
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
