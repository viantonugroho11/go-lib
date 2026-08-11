package kafka

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

// Consumer is the interface for a Kafka consumer group. Returned by NewConsumer.
type Consumer interface {
	Start(ctx context.Context)
	Close() error
}

// messageResult signals to the sarama loop what to do with the message after the handler ran.
type messageResult struct {
	err   error // wrapped for logging only; MarkMessage decision is separate
	drop  bool  // true = do not MarkMessage
	dlq   *DeadMessage
	ctx   context.Context
}

type messageHandler func(ctx context.Context, msg *sarama.ConsumerMessage) messageResult

type consumer struct {
	group       sarama.ConsumerGroup
	topics      []string
	handler     messageHandler
	logger      Logger
	strategy    ErrorStrategy
	dlq         DeadLetterFunc
	backoff     BlockBackoff
	concurrency int

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewConsumer creates a consumer group for one topic with EventHandler[E] (JSON decode by default).
// Use WithHeaderKeys to pass selected headers into Handle; omit for no headers.
// Error strategy defaults to ErrorSkip; set WithErrorStrategy + WithDeadLetter for DLQ, or WithBlockOnError for retry.
func NewConsumer[E any](brokers []string, groupID string, topic string, handler EventHandler[E], options ...ConsumerOption) (Consumer, error) {
	if topic == "" {
		return nil, errors.New("kafka: topic required")
	}
	cfg := &consumerBuildConfig{
		cfg:         defaultSaramaConfig(),
		logger:      stderrLogger{},
		strategy:    ErrorSkip,
		backoff:     defaultBlockBackoff(),
		concurrency: 1,
	}
	applyConsumerOptions(cfg, options)
	opts := []handlerOption[E]{withJSONDecoder[E]()}
	if cfg.decoder != nil {
		if fn, ok := cfg.decoder.(func([]byte, *E) error); ok {
			opts = append(opts, withDecoder(fn))
		}
	}
	adapted := adaptEventHandler(handler, cfg.headerKeys, opts...)
	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg.cfg)
	if err != nil {
		return nil, err
	}
	return &consumer{
		group:       group,
		topics:      []string{topic},
		handler:     adapted,
		logger:      cfg.logger,
		strategy:    cfg.strategy,
		dlq:         cfg.dlq,
		backoff:     cfg.backoff,
		concurrency: cfg.concurrency,
	}, nil
}

func (c *consumer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.wg.Add(2)
	go func() {
		defer c.wg.Done()
		for err := range c.group.Errors() {
			if err != nil {
				c.logger.Errorf(ctx, "consumer group error: %v", err)
			}
		}
	}()
	go func() {
		defer c.wg.Done()
		for {
			if err := c.group.Consume(ctx, c.topics, c.newHandler()); err != nil {
				if errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
				c.logger.Errorf(ctx, "consume loop error: %v", err)
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
}

func (c *consumer) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	err := c.group.Close() // must precede wg.Wait: drains Errors() channel so error goroutine exits
	c.wg.Wait()
	return err
}

func (c *consumer) newHandler() *cgHandler {
	return &cgHandler{
		handler:     c.handler,
		logger:      c.logger,
		strategy:    c.strategy,
		dlq:         c.dlq,
		backoff:     c.backoff,
		concurrency: c.concurrency,
	}
}

type cgHandler struct {
	handler     messageHandler
	logger      Logger
	strategy    ErrorStrategy
	dlq         DeadLetterFunc
	backoff     BlockBackoff
	concurrency int
}

func (h *cgHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *cgHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *cgHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	if h.concurrency <= 1 {
		for msg := range claim.Messages() {
			h.processOne(sess, msg)
		}
		return nil
	}
	h.consumeClaimPool(sess, claim)
	return nil
}

// consumeClaimPool runs N workers per partition claim. Completed offsets go through a
// contiguous-commit tracker so we never mark past a hole (an uncommitted lower offset).
// On rebalance, any messages past a hole get re-delivered — at-least-once preserved.
func (h *cgHandler) consumeClaimPool(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) {
	jobs := make(chan *sarama.ConsumerMessage, h.concurrency)
	done := make(chan *sarama.ConsumerMessage, h.concurrency)

	var wg sync.WaitGroup
	for i := 0; i < h.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msg := range jobs {
				h.processOneNoMark(sess, msg)
				done <- msg
			}
		}()
	}

	commitDone := make(chan struct{})
	go func() {
		defer close(commitDone)
		completed := make(map[int64]*sarama.ConsumerMessage)
		nextExpected := int64(-1)
		for msg := range done {
			completed[msg.Offset] = msg
			if nextExpected == -1 {
				nextExpected = msg.Offset
			}
			var latest *sarama.ConsumerMessage
			for {
				m, ok := completed[nextExpected]
				if !ok {
					break
				}
				latest = m
				delete(completed, nextExpected)
				nextExpected++
			}
			if latest != nil {
				sess.MarkMessage(latest, "")
			}
		}
	}()

	for msg := range claim.Messages() {
		jobs <- msg
	}
	close(jobs)
	wg.Wait()
	close(done)
	<-commitDone
}

// processOneNoMark runs the handler + error strategy without calling MarkMessage.
// The pool committer is responsible for marking once contiguous ordering is safe.
func (h *cgHandler) processOneNoMark(sess sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	attempt := 0
	var backoff time.Duration
	for {
		attempt++
		res := h.handler(sess.Context(), msg)
		if res.err == nil {
			return
		}
		switch h.strategy {
		case ErrorBlock:
			h.logger.Errorf(res.ctx, "handler error (attempt %d), blocking retry: %v", attempt, res.err)
			backoff = h.backoff.next(backoff)
			select {
			case <-sess.Context().Done():
				return
			case <-time.After(backoff):
			}
			continue
		case ErrorDeadLetter:
			if h.dlq == nil {
				h.logger.Errorf(res.ctx, "handler error, DLQ not configured — falling back to skip: %v", res.err)
				return
			}
			dm := DeadMessage{
				Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset,
				Key: msg.Value, Value: msg.Value, Headers: headersFromMessage(msg),
				Err: res.err, Attempt: attempt,
			}
			if msg.Key != nil {
				dm.Key = msg.Key
			}
			if err := h.dlq(sess.Context(), dm); err != nil {
				h.logger.Errorf(res.ctx, "DLQ publish failed, falling back to skip: %v (original: %v)", err, res.err)
			}
			return
		default:
			h.logger.Errorf(res.ctx, "handler error, skipping (offset advances): %v", res.err)
			return
		}
	}
}

// processOne runs the handler and applies the configured error strategy.
// Contract: MarkMessage is called ONLY when we intend to commit past this offset.
// ErrorBlock loops on the same message; ErrorSkip / successful DLQ / success all mark.
func (h *cgHandler) processOne(sess sarama.ConsumerGroupSession, msg *sarama.ConsumerMessage) {
	attempt := 0
	var backoff time.Duration
	for {
		attempt++
		res := h.handler(sess.Context(), msg)
		if res.err == nil {
			sess.MarkMessage(msg, "")
			return
		}

		switch h.strategy {
		case ErrorBlock:
			h.logger.Errorf(res.ctx, "handler error (attempt %d), blocking retry: %v", attempt, res.err)
			backoff = h.backoff.next(backoff)
			select {
			case <-sess.Context().Done():
				return
			case <-time.After(backoff):
			}
			continue

		case ErrorDeadLetter:
			if h.dlq == nil {
				h.logger.Errorf(res.ctx, "handler error, DLQ not configured — falling back to skip: %v", res.err)
				sess.MarkMessage(msg, "")
				return
			}
			dm := DeadMessage{
				Topic: msg.Topic, Partition: msg.Partition, Offset: msg.Offset,
				Key: msg.Value, Value: msg.Value, Headers: headersFromMessage(msg),
				Err: res.err, Attempt: attempt,
			}
			if msg.Key != nil {
				dm.Key = msg.Key
			}
			if err := h.dlq(sess.Context(), dm); err != nil {
				h.logger.Errorf(res.ctx, "DLQ publish failed, falling back to skip: %v (original: %v)", err, res.err)
			}
			sess.MarkMessage(msg, "")
			return

		default: // ErrorSkip
			h.logger.Errorf(res.ctx, "handler error, skipping (offset advances): %v", res.err)
			sess.MarkMessage(msg, "")
			return
		}
	}
}
