package kafka

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

// --- mock session/claim implementing sarama.ConsumerGroupSession + ConsumerGroupClaim ---

type mockSession struct {
	ctx    context.Context
	marked []int64
	mu     sync.Mutex
}

func (m *mockSession) Claims() map[string][]int32                                              { return nil }
func (m *mockSession) MemberID() string                                                        { return "test" }
func (m *mockSession) GenerationID() int32                                                     { return 0 }
func (m *mockSession) MarkOffset(_ string, _ int32, _ int64, _ string)                         {}
func (m *mockSession) ResetOffset(_ string, _ int32, _ int64, _ string)                        {}
func (m *mockSession) Commit()                                                                 {}
func (m *mockSession) Context() context.Context                                                { return m.ctx }
func (m *mockSession) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	m.mu.Lock()
	m.marked = append(m.marked, msg.Offset)
	m.mu.Unlock()
}
func (m *mockSession) markedOffsets() []int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]int64, len(m.marked))
	copy(out, m.marked)
	return out
}

// --- test event + handler ---

type testEvent struct {
	ID string `json:"id"`
}

type recordingHandler struct {
	name  string
	fn    func(testEvent) Progress
	calls int64

	mu   sync.Mutex
	seen []testEvent
}

func (h *recordingHandler) Name() string { return h.name }
func (h *recordingHandler) Handle(_ context.Context, evt testEvent, _ ...Header) Progress {
	atomic.AddInt64(&h.calls, 1)
	h.mu.Lock()
	h.seen = append(h.seen, evt)
	h.mu.Unlock()
	if h.fn != nil {
		return h.fn(evt)
	}
	return Progress{Status: ProgressSuccess}
}

func newMsg(offset int64, val string) *sarama.ConsumerMessage {
	return &sarama.ConsumerMessage{
		Topic:     "t",
		Partition: 0,
		Offset:    offset,
		Value:     []byte(`{"id":"` + val + `"}`),
	}
}

// buildProcessor mirrors what NewConsumer would wire, without opening a real ConsumerGroup.
func buildProcessor(t *testing.T, handler EventHandler[testEvent], strat ErrorStrategy, dlq DeadLetterFunc) (*cgHandler, *mockSession) {
	t.Helper()
	sess := &mockSession{ctx: context.Background()}
	proc := &cgHandler{
		handler:  adaptEventHandler(handler, nil, withJSONDecoder[testEvent]()),
		logger:   stderrLogger{},
		strategy: strat,
		dlq:      dlq,
		backoff:  BlockBackoff{Initial: 1 * time.Millisecond, Max: 5 * time.Millisecond, Factor: 2},
	}
	return proc, sess
}

// --- tests ---

func TestSuccessMarksOffset(t *testing.T) {
	h := &recordingHandler{name: "ok"}
	proc, sess := buildProcessor(t, h, ErrorSkip, nil)
	proc.processOne(sess, newMsg(10, "a"))
	if got := sess.markedOffsets(); len(got) != 1 || got[0] != 10 {
		t.Fatalf("marked = %v, want [10]", got)
	}
}

// TestErrorSkipAdvancesOffset documents the OLD (v0.1.x) behavior — now explicit and logged.
func TestErrorSkipAdvancesOffset(t *testing.T) {
	h := &recordingHandler{
		name: "err",
		fn: func(evt testEvent) Progress {
			p := Progress{}
			p.SetError(errors.New("boom"))
			return p
		},
	}
	proc, sess := buildProcessor(t, h, ErrorSkip, nil)
	proc.processOne(sess, newMsg(5, "a"))
	if got := sess.markedOffsets(); len(got) != 1 || got[0] != 5 {
		t.Fatalf("marked = %v, want [5] (skip advances)", got)
	}
}

// TestErrorBlockRetriesUntilSuccess proves that ErrorBlock keeps invoking until handler succeeds.
func TestErrorBlockRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	h := &recordingHandler{
		name: "block",
		fn: func(evt testEvent) Progress {
			attempts++
			if attempts < 3 {
				p := Progress{}
				p.SetError(errors.New("try again"))
				return p
			}
			return Progress{Status: ProgressSuccess}
		},
	}
	proc, sess := buildProcessor(t, h, ErrorBlock, nil)
	proc.processOne(sess, newMsg(7, "a"))
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if got := sess.markedOffsets(); len(got) != 1 || got[0] != 7 {
		t.Fatalf("marked = %v, want [7]", got)
	}
}

// TestErrorBlockRespectsContextCancel proves that ErrorBlock exits when session ctx cancels.
func TestErrorBlockRespectsContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	h := &recordingHandler{
		name: "forever",
		fn: func(_ testEvent) Progress {
			p := Progress{}
			p.SetError(errors.New("never succeeds"))
			return p
		},
	}
	sess := &mockSession{ctx: ctx}
	proc := &cgHandler{
		handler:  adaptEventHandler(h, nil, withJSONDecoder[testEvent]()),
		logger:   stderrLogger{},
		strategy: ErrorBlock,
		backoff:  BlockBackoff{Initial: 1 * time.Millisecond, Max: 5 * time.Millisecond, Factor: 2},
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	done := make(chan struct{})
	go func() {
		proc.processOne(sess, newMsg(1, "x"))
		close(done)
	}()
	select {
	case <-done:
		if got := sess.markedOffsets(); len(got) != 0 {
			t.Fatalf("marked = %v, want [] (ctx cancelled without success)", got)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("processOne did not exit on ctx cancel")
	}
}

// TestErrorDeadLetterRoutesThenMarks proves DLQ receives the failed message and offset advances.
func TestErrorDeadLetterRoutesThenMarks(t *testing.T) {
	var got DeadMessage
	var called int32
	dlq := func(_ context.Context, m DeadMessage) error {
		atomic.AddInt32(&called, 1)
		got = m
		return nil
	}
	h := &recordingHandler{
		name: "dlq",
		fn: func(_ testEvent) Progress {
			p := Progress{}
			p.SetError(errors.New("bad data"))
			return p
		},
	}
	proc, sess := buildProcessor(t, h, ErrorDeadLetter, dlq)
	msg := newMsg(42, "z")
	msg.Key = []byte("k")
	proc.processOne(sess, msg)
	if atomic.LoadInt32(&called) != 1 {
		t.Fatalf("dlq called %d times, want 1", called)
	}
	if got.Offset != 42 || string(got.Key) != "k" || got.Err == nil {
		t.Fatalf("bad dlq message: %+v", got)
	}
	if marked := sess.markedOffsets(); len(marked) != 1 || marked[0] != 42 {
		t.Fatalf("marked = %v, want [42]", marked)
	}
}

// TestErrorDeadLetterFallsBackToSkipWhenDLQFails proves DLQ publish error → skip (log + advance).
func TestErrorDeadLetterFallsBackToSkipWhenDLQFails(t *testing.T) {
	dlq := func(_ context.Context, _ DeadMessage) error { return errors.New("dlq down") }
	h := &recordingHandler{
		name: "dlq-fail",
		fn: func(_ testEvent) Progress {
			p := Progress{}
			p.SetError(errors.New("boom"))
			return p
		},
	}
	proc, sess := buildProcessor(t, h, ErrorDeadLetter, dlq)
	proc.processOne(sess, newMsg(99, "z"))
	if marked := sess.markedOffsets(); len(marked) != 1 || marked[0] != 99 {
		t.Fatalf("marked = %v, want [99] (fallback)", marked)
	}
}

// TestJSONDecodeFailureCountsAsError verifies malformed payload triggers error strategy.
func TestJSONDecodeFailureCountsAsError(t *testing.T) {
	h := &recordingHandler{name: "decode"}
	proc, sess := buildProcessor(t, h, ErrorSkip, nil)
	bad := &sarama.ConsumerMessage{Topic: "t", Offset: 3, Value: []byte("not json")}
	proc.processOne(sess, bad)
	if atomic.LoadInt64(&h.calls) != 0 {
		t.Fatalf("handler called %d times, want 0", h.calls)
	}
	if marked := sess.markedOffsets(); len(marked) != 1 || marked[0] != 3 {
		t.Fatalf("marked = %v, want [3] (skip on decode fail)", marked)
	}
}

func TestHeaderKeysFilter(t *testing.T) {
	var got []Header
	h := &recordingHandler{
		name: "hdr",
		fn: func(_ testEvent) Progress { return Progress{Status: ProgressSuccess} },
	}
	proc := &cgHandler{
		handler:  adaptEventHandler[testEvent](captureHeaders(h, &got), []string{"trace_id"}, withJSONDecoder[testEvent]()),
		logger:   stderrLogger{},
		strategy: ErrorSkip,
	}
	sess := &mockSession{ctx: context.Background()}
	msg := newMsg(1, "a")
	msg.Headers = []*sarama.RecordHeader{
		{Key: []byte("trace_id"), Value: []byte("t1")},
		{Key: []byte("noisy"), Value: []byte("drop")},
	}
	proc.processOne(sess, msg)
	if len(got) != 1 || got[0].Key != "trace_id" || string(got[0].Value) != "t1" {
		t.Fatalf("filtered headers = %+v", got)
	}
}

// captureHeaders wraps a handler to record headers seen.
func captureHeaders(inner EventHandler[testEvent], sink *[]Header) EventHandler[testEvent] {
	return headerCapture{inner: inner, sink: sink}
}

type headerCapture struct {
	inner EventHandler[testEvent]
	sink  *[]Header
}

func (h headerCapture) Name() string { return h.inner.Name() }
func (h headerCapture) Handle(ctx context.Context, evt testEvent, headers ...Header) Progress {
	*h.sink = append(*h.sink, headers...)
	return h.inner.Handle(ctx, evt, headers...)
}
