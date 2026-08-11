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

// injectTrace serializes the ctx trace context into headers using the global TextMapPropagator.
// No-op when no propagator is configured (default global is a no-op).
func injectTrace(ctx context.Context, headers []Header) []Header {
	prop := otel.GetTextMapPropagator()
	carrier := headerCarrier{headers: &headers}
	prop.Inject(ctx, carrier)
	return headers
}

// extractTrace returns a ctx carrying any propagated trace context from headers.
func extractTrace(ctx context.Context, headers []Header) context.Context {
	prop := otel.GetTextMapPropagator()
	carrier := headerCarrier{headers: &headers}
	return prop.Extract(ctx, carrier)
}

var _ propagation.TextMapCarrier = headerCarrier{}
