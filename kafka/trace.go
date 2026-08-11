package kafka

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// headerCarrier adapts []Header to propagation.TextMapCarrier.
type headerCarrier struct {
	headers *[]Header
}

func (c headerCarrier) Get(key string) string {
	for _, h := range *c.headers {
		if h.Key == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c headerCarrier) Set(key, value string) {
	for i, h := range *c.headers {
		if h.Key == key {
			(*c.headers)[i].Value = []byte(value)
			return
		}
	}
	*c.headers = append(*c.headers, Header{Key: key, Value: []byte(value)})
}

func (c headerCarrier) Keys() []string {
	out := make([]string, 0, len(*c.headers))
	for _, h := range *c.headers {
		out = append(out, h.Key)
	}
	return out
}

// tracingActive reports whether the global TextMapPropagator would actually write/read
// any keys. When empty (the default), inject/extract can be skipped for a real speedup.
func tracingActive() bool {
	return len(otel.GetTextMapPropagator().Fields()) > 0
}

// injectTrace serializes the ctx trace context into headers using the global TextMapPropagator.
// Fast-return when the propagator is empty (default global), avoiding carrier allocation.
func injectTrace(ctx context.Context, headers []Header) []Header {
	if !tracingActive() {
		return headers
	}
	carrier := headerCarrier{headers: &headers}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return headers
}

// extractTrace returns a ctx carrying any propagated trace context from headers.
// Fast-return when the propagator is empty.
func extractTrace(ctx context.Context, headers []Header) context.Context {
	if !tracingActive() {
		return ctx
	}
	carrier := headerCarrier{headers: &headers}
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}

var _ propagation.TextMapCarrier = headerCarrier{}
