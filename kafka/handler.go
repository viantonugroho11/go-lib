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

func filterHeadersByKeys(headers []Header, keys []string) []Header {
	if len(keys) == 0 || len(headers) == 0 {
		return nil
	}
	allowed := make(map[string]bool, len(keys))
	for _, k := range keys {
		allowed[k] = true
	}
	out := make([]Header, 0, len(headers))
	for _, h := range headers {
		if allowed[h.Key] {
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
	return func(ctx context.Context, msg *sarama.ConsumerMessage) messageResult {
		all := headersFromMessage(msg)
		// Extract trace context (traceparent etc.) from ALL headers before filtering.
		ctx = extractTrace(ctx, all)

		evt := cfg.newEvent()
		if err := cfg.decode(msg.Value, &evt); err != nil {
			return messageResult{err: err, ctx: ctx}
		}
		headers := filterHeadersByKeys(all, headerKeys)
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
