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
	group      sarama.ConsumerGroup
	topics     []string
	handler    messageHandler
	logger     Logger
	strategy   ErrorStrategy
	dlq        DeadLetterFunc
	backoff    BlockBackoff

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewConsumer creates a consumer group over one or more topics with EventHandler[E] (JSON decode).
// Use WithHeaderKeys to pass selected headers into Handle; omit for no headers.
// Error strategy defaults to ErrorSkip; set WithErrorStrategy + WithDeadLetter for DLQ, or WithBlockOnError for retry.
func NewConsumer[E any](brokers []string, groupID string, topics []string, handler EventHandler[E], options ...ConsumerOption) (Consumer, error) {
	if len(topics) == 0 {
		return nil, errors.New("kafka: at least one topic required")
	}
	cfg := &consumerBuildConfig{
		cfg:      defaultSaramaConfig(),
		logger:   stderrLogger{},
		strategy: ErrorSkip,
		backoff:  defaultBlockBackoff(),
	}
	applyConsumerOptions(cfg, options)
	adapted := adaptEventHandler(handler, cfg.headerKeys, withJSONDecoder[E]())
	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg.cfg)
	if err != nil {
		return nil, err
	}
	return &consumer{
		group:    group,
		topics:   topics,
		handler:  adapted,
		logger:   cfg.logger,
		strategy: cfg.strategy,
		dlq:      cfg.dlq,
		backoff:  cfg.backoff,
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
		handler:  c.handler,
		logger:   c.logger,
		strategy: c.strategy,
		dlq:      c.dlq,
		backoff:  c.backoff,
	}
}

type cgHandler struct {
	handler  messageHandler
	logger   Logger
	strategy ErrorStrategy
	dlq      DeadLetterFunc
	backoff  BlockBackoff
}

func (h *cgHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *cgHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *cgHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		h.processOne(sess, msg)
	}
	return nil
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
