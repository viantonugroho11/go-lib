package kafka

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestTraceRoundTrip(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	defer otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())

	traceID, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	spanID, _ := trace.SpanIDFromHex("b7ad6b7169203331")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: false,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	headers := injectTrace(ctx, nil)
	if len(headers) == 0 {
		t.Fatal("no headers injected")
	}
	var haveTraceparent bool
	for _, h := range headers {
		if h.Key == "traceparent" {
			haveTraceparent = true
		}
	}
	if !haveTraceparent {
		t.Fatalf("no traceparent header: %+v", headers)
	}

	got := extractTrace(context.Background(), headers)
	extracted := trace.SpanContextFromContext(got)
	if !extracted.IsValid() {
		t.Fatal("extracted span context invalid")
	}
	if extracted.TraceID() != traceID {
		t.Fatalf("trace id = %s, want %s", extracted.TraceID(), traceID)
	}
	if extracted.SpanID() != spanID {
		t.Fatalf("span id = %s, want %s", extracted.SpanID(), spanID)
	}
}

func TestInjectTraceNoOpWithoutPropagator(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
	headers := injectTrace(context.Background(), []Header{{Key: "x", Value: []byte("y")}})
	if len(headers) != 1 || headers[0].Key != "x" {
		t.Fatalf("no-op corrupted headers: %+v", headers)
	}
}
