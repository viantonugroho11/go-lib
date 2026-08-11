package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// --- hot path: adapter (decode + trace extract + filter + handle) ---

type noopHandler struct{}

func (noopHandler) Name() string { return "n" }
func (noopHandler) Handle(_ context.Context, _ testEvent, _ ...Header) Progress {
	return Progress{Status: ProgressSuccess}
}

func BenchmarkAdapter_NoHeaders(b *testing.B) {
	adapter := adaptEventHandler[testEvent](noopHandler{}, nil, withJSONDecoder[testEvent]())
	payload, _ := json.Marshal(testEvent{ID: "abc-123-xyz"})
	msg := &sarama.ConsumerMessage{Topic: "t", Offset: 1, Value: payload}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter(ctx, msg)
	}
}

func BenchmarkAdapter_WithHeadersFiltered(b *testing.B) {
	adapter := adaptEventHandler[testEvent](noopHandler{}, []string{"trace_id", "correlation_id"}, withJSONDecoder[testEvent]())
	payload, _ := json.Marshal(testEvent{ID: "abc-123-xyz"})
	msg := &sarama.ConsumerMessage{
		Topic: "t", Offset: 1, Value: payload,
		Headers: []*sarama.RecordHeader{
			{Key: []byte("trace_id"), Value: []byte("t-abc")},
			{Key: []byte("correlation_id"), Value: []byte("c-xyz")},
			{Key: []byte("noise-1"), Value: []byte("drop-1")},
			{Key: []byte("noise-2"), Value: []byte("drop-2")},
			{Key: []byte("noise-3"), Value: []byte("drop-3")},
		},
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = adapter(ctx, msg)
	}
}

// --- trace propagation ---

func BenchmarkInjectTrace_NoPropagator(b *testing.B) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = injectTrace(ctx, nil)
	}
}

func BenchmarkInjectTrace_TraceContextPropagator(b *testing.B) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	tid, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	sid, _ := trace.SpanIDFromHex("b7ad6b7169203331")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: tid, SpanID: sid, TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = injectTrace(ctx, nil)
	}
}

func BenchmarkExtractTrace_TraceContextPropagator(b *testing.B) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	headers := []Header{
		{Key: "traceparent", Value: []byte("00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")},
		{Key: "tracestate", Value: []byte("vendor=foo")},
	}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = extractTrace(ctx, headers)
	}
}

// --- header helpers ---

func BenchmarkFilterHeadersByKeys(b *testing.B) {
	headers := []Header{
		{Key: "trace_id", Value: []byte("t")},
		{Key: "correlation_id", Value: []byte("c")},
		{Key: "noise-1", Value: []byte("x")},
		{Key: "noise-2", Value: []byte("x")},
		{Key: "noise-3", Value: []byte("x")},
		{Key: "noise-4", Value: []byte("x")},
	}
	keys := []string{"trace_id", "correlation_id"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = filterHeadersByKeys(headers, keys)
	}
}

func BenchmarkHeadersFromMessage(b *testing.B) {
	msg := &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{Key: []byte("trace_id"), Value: []byte("t")},
			{Key: []byte("correlation_id"), Value: []byte("c")},
			{Key: []byte("k3"), Value: []byte("v3")},
			{Key: []byte("k4"), Value: []byte("v4")},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = headersFromMessage(msg)
	}
}

func BenchmarkToRecordHeaders(b *testing.B) {
	headers := []Header{
		{Key: "traceparent", Value: []byte("00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01")},
		{Key: "correlation_id", Value: []byte("c-xyz")},
		{Key: "tenant", Value: []byte("t-1")},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = toRecordHeaders(headers)
	}
}

// --- JSON encode (dominant producer cost) ---

func BenchmarkJSONEncode(b *testing.B) {
	v := testEvent{ID: "some-reasonable-id-length"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(v)
	}
}

// --- pool committer path in isolation ---

func BenchmarkPoolCommitter_Serial(b *testing.B) {
	// Baseline: concurrency=1 (serial path).
	h := &recordingHandler{name: "s"}
	proc := &cgHandler{
		handler:     adaptEventHandler(h, nil, withJSONDecoder[testEvent]()),
		logger:      stderrLogger{},
		strategy:    ErrorSkip,
		concurrency: 1,
	}
	msgs := makeMsgs(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sess := &mockSession{ctx: context.Background()}
		claim := &mockClaim{msgs: make(chan *sarama.ConsumerMessage, len(msgs))}
		for _, m := range msgs {
			claim.msgs <- m
		}
		close(claim.msgs)
		_ = proc.ConsumeClaim(sess, claim)
	}
}

func BenchmarkPoolCommitter_4Workers(b *testing.B) {
	h := &recordingHandler{name: "p"}
	proc := &cgHandler{
		handler:     adaptEventHandler(h, nil, withJSONDecoder[testEvent]()),
		logger:      stderrLogger{},
		strategy:    ErrorSkip,
		concurrency: 4,
	}
	msgs := makeMsgs(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sess := &mockSession{ctx: context.Background()}
		claim := &mockClaim{msgs: make(chan *sarama.ConsumerMessage, len(msgs))}
		for _, m := range msgs {
			claim.msgs <- m
		}
		close(claim.msgs)
		_ = proc.ConsumeClaim(sess, claim)
	}
}

func makeMsgs(n int) []*sarama.ConsumerMessage {
	out := make([]*sarama.ConsumerMessage, n)
	for i := 0; i < n; i++ {
		payload, _ := json.Marshal(testEvent{ID: "e"})
		out[i] = &sarama.ConsumerMessage{Topic: "t", Partition: 0, Offset: int64(i), Value: payload}
	}
	return out
}
