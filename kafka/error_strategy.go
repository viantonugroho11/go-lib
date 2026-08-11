package kafka

import (
	"context"
	"time"
)

// ErrorStrategy decides what the consumer does when a handler returns ProgressError.
type ErrorStrategy int

const (
	// ErrorSkip advances the offset (message dropped after logging). Default.
	ErrorSkip ErrorStrategy = iota
	// ErrorBlock re-invokes the handler with backoff until success or ctx cancellation.
	// One slow message will block its partition.
	ErrorBlock
	// ErrorDeadLetter publishes the failed message to a DLQ via DeadLetterFunc, then advances the offset.
	// Requires WithDeadLetter to be set; falls back to ErrorSkip otherwise.
	ErrorDeadLetter
)

// DeadMessage is the payload delivered to DeadLetterFunc.
type DeadMessage struct {
	Topic     string
	Partition int32
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   []Header
	Err       error
	Attempt   int
}

// DeadLetterFunc publishes a failed message to a DLQ. Return non-nil to signal DLQ publish failure —
// the consumer will fall back to ErrorSkip (log + advance) so the pipeline does not stall.
type DeadLetterFunc func(ctx context.Context, msg DeadMessage) error

// BlockBackoff configures ErrorBlock retry pacing.
type BlockBackoff struct {
	Initial time.Duration
	Max     time.Duration
	Factor  float64
}

func (b BlockBackoff) next(prev time.Duration) time.Duration {
	if prev == 0 {
		return b.Initial
	}
	next := time.Duration(float64(prev) * b.Factor)
	if next > b.Max {
		return b.Max
	}
	return next
}

func defaultBlockBackoff() BlockBackoff {
	return BlockBackoff{Initial: 100 * time.Millisecond, Max: 30 * time.Second, Factor: 2.0}
}
