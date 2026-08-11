package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/IBM/sarama"
)

// AsyncCallback is invoked from a dedicated goroutine after a message is acked (err==nil)
// or fails. It must be non-blocking; long work will backpressure the delivery drain.
// The value is echoed back so callers can correlate; ctx carries the trace context
// used at Publish time.
type AsyncCallback[T any] func(ctx context.Context, value T, err error)

// AsyncProducer[T] wraps sarama.AsyncProducer. Publish returns immediately after the
// message is enqueued to sarama's input channel; delivery outcome is signaled via
// the callback registered with WithAsyncCallback.
type AsyncProducer[T any] struct {
	ap           sarama.AsyncProducer
	topicName    string
	key          []byte
	keyExtractor func(T) []byte
	encode       func(T) ([]byte, error)
	callback     AsyncCallback[T]
	logger       Logger

	closeOnce sync.Once
	closed    chan struct{}
	drainWG   sync.WaitGroup
}

// asyncMetadata pairs the outbound message with its origin ctx + value for callback delivery.
type asyncMetadata[T any] struct {
	ctx   context.Context
	value T
}

// NewAsyncProducer creates an AsyncProducer[T] bound to one topic and one message type T.
// Delivery outcomes are signaled through WithAsyncCallback; without a callback, errors
// are logged and successes are silent. Trace context is injected on Publish.
func NewAsyncProducer[T any](brokers []string, topic string, options ...ProducerOption) (*AsyncProducer[T], error) {
	if topic == "" {
		return nil, errors.New("kafka: topic required")
	}
	build := &producerBuildConfig{
		cfg:    defaultProducerConfig(),
		logger: stderrLogger{},
	}
	for _, opt := range options {
		opt.apply(build)
	}
	// AsyncProducer must have both Return.Successes and Return.Errors enabled for callback fan-out.
	build.cfg.Producer.Return.Successes = true
	build.cfg.Producer.Return.Errors = true

	ap, err := sarama.NewAsyncProducer(brokers, build.cfg)
	if err != nil {
		return nil, err
	}
	var keyExtractor func(T) []byte
	if build.keyExtractor != nil {
		keyExtractor, _ = build.keyExtractor.(func(T) []byte)
	}
	var encode func(T) ([]byte, error)
	if build.encoder != nil {
		if fn, ok := build.encoder.(func(T) ([]byte, error)); ok {
			encode = fn
		}
	}
	if encode == nil {
		encode = func(v T) ([]byte, error) { return json.Marshal(v) }
	}
	var cb AsyncCallback[T]
	if build.asyncCallback != nil {
		if fn, ok := build.asyncCallback.(AsyncCallback[T]); ok {
			cb = fn
		}
	}

	p := &AsyncProducer[T]{
		ap:           ap,
		topicName:    topic,
		key:          build.key,
		keyExtractor: keyExtractor,
		encode:       encode,
		callback:     cb,
		logger:       build.logger,
		closed:       make(chan struct{}),
	}
	p.drainWG.Add(2)
	go p.drainSuccesses()
	go p.drainErrors()
	return p, nil
}

// Publish enqueues one message. Returns as soon as sarama accepts it into its input channel.
// Delivery outcome is delivered later via the async callback (if set). Returns an error only
// on encoder failure or if the producer is closed.
func (p *AsyncProducer[T]) Publish(ctx context.Context, value T, headers ...Header) error {
	select {
	case <-p.closed:
		return errors.New("kafka: async producer closed")
	default:
	}
	encoded, err := p.encode(value)
	if err != nil {
		return err
	}
	key := p.key
	if p.keyExtractor != nil {
		key = p.keyExtractor(value)
	}
	msg := &sarama.ProducerMessage{
		Topic:    p.topicName,
		Key:      sarama.ByteEncoder(key),
		Value:    sarama.ByteEncoder(encoded),
		Headers:  toRecordHeaders(injectTrace(ctx, cloneHeaders(headers))),
		Metadata: asyncMetadata[T]{ctx: ctx, value: value},
	}
	p.ap.Input() <- msg
	return nil
}

// Close flushes in-flight messages and closes the underlying producer.
// Callbacks for messages already accepted before Close will still fire.
func (p *AsyncProducer[T]) Close() error {
	var err error
	p.closeOnce.Do(func() {
		close(p.closed)
		err = p.ap.Close() // blocks until in-flight messages are drained + Successes/Errors channels closed
		p.drainWG.Wait()
	})
	return err
}

func (p *AsyncProducer[T]) drainSuccesses() {
	defer p.drainWG.Done()
	for msg := range p.ap.Successes() {
		p.fireCallback(msg, nil)
	}
}

func (p *AsyncProducer[T]) drainErrors() {
	defer p.drainWG.Done()
	for perr := range p.ap.Errors() {
		if p.callback == nil && p.logger != nil {
			p.logger.Errorf(context.Background(), "async producer error: %v", perr.Err)
		}
		p.fireCallback(perr.Msg, perr.Err)
	}
}

func (p *AsyncProducer[T]) fireCallback(msg *sarama.ProducerMessage, err error) {
	if p.callback == nil || msg == nil {
		return
	}
	meta, ok := msg.Metadata.(asyncMetadata[T])
	if !ok {
		return
	}
	p.callback(meta.ctx, meta.value, err)
}
