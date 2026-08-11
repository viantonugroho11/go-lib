package kafka

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/IBM/sarama"
)

type handlerOption[E any] func(*handlerConfig[E])

type handlerConfig[E any] struct {
	newEvent func() E
	decode   func([]byte, *E) error
}

func withJSONDecoder[E any]() handlerOption[E] {
	return func(c *handlerConfig[E]) {
		c.decode = func(b []byte, dst *E) error { return json.Unmarshal(b, dst) }
	}
}

// withDecoder is the internal wiring used by WithDecoder[E].
func withDecoder[E any](fn func([]byte, *E) error) handlerOption[E] {
	return func(c *handlerConfig[E]) { c.decode = fn }
}

func headersFromMessage(msg *sarama.ConsumerMessage) []Header {
	if len(msg.Headers) == 0 {
		return nil
	}
	out := make([]Header, 0, len(msg.Headers))
	for _, h := range msg.Headers {
		if h != nil {
			out = append(out, Header{Key: string(h.Key), Value: h.Value})
		}
	}
	return out
}

// filterHeadersByKeys returns only the headers whose Key appears in keys.
// For len(keys) <= 4 (the common case: trace_id, correlation_id, tenant, user_id),
// use a linear scan — zero map allocation and faster on modern CPUs than hashing.
// Above the threshold, build a lookup map.
func filterHeadersByKeys(headers []Header, keys []string) []Header {
	if len(keys) == 0 || len(headers) == 0 {
		return nil
	}
	out := make([]Header, 0, len(keys))
	if len(keys) <= 4 {
		for _, h := range headers {
			for _, k := range keys {
				if h.Key == k {
					out = append(out, h)
					break
				}
			}
		}
		return out
	}
	allowed := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		allowed[k] = struct{}{}
	}
	for _, h := range headers {
		if _, ok := allowed[h.Key]; ok {
			out = append(out, h)
		}
	}
	return out
}

func adaptEventHandler[E any](handler EventHandler[E], headerKeys []string, opts ...handlerOption[E]) messageHandler {
	cfg := &handlerConfig[E]{
		newEvent: func() E { var zero E; return zero },
		decode:   func(b []byte, dst *E) error { return json.Unmarshal(b, dst) },
	}
	for _, o := range opts {
		o(cfg)
	}
	wantHeaders := len(headerKeys) > 0
	// Cache the propagator-active check at adapter construction time; otel.GetTextMapPropagator().Fields()
	// allocates on every call (composite propagator builds a map). If the user swaps propagators after
	// starting the consumer, they must restart it — pragmatically no one does.
	tracingAtInit := tracingActive()
	return func(ctx context.Context, msg *sarama.ConsumerMessage) messageResult {
		var headers []Header
		if wantHeaders || tracingAtInit {
			if len(msg.Headers) > 0 {
				all := headersFromMessage(msg)
				if tracingAtInit {
					ctx = extractTrace(ctx, all)
				}
				if wantHeaders {
					headers = filterHeadersByKeys(all, headerKeys)
				}
			}
		}

		evt := cfg.newEvent()
		if err := cfg.decode(msg.Value, &evt); err != nil {
			return messageResult{err: err, ctx: ctx}
		}
		progress := handler.Handle(ctx, evt, headers...)
		if progress.Status == ProgressError {
			err := progress.Err
			if err == nil {
				err = errors.New(progress.Result)
			}
			if err == nil {
				err = errors.New("progress error without message")
			}
			return messageResult{err: err, ctx: ctx}
		}
		return messageResult{ctx: ctx}
	}
}
