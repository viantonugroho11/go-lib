package kafka

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/IBM/sarama"
)

// Consumer is the interface for a Kafka consumer group. Returned by NewConsumer.
type Consumer interface {
	Start(ctx context.Context)
	Close() error
}

type messageHandler func(ctx context.Context, msg *sarama.ConsumerMessage) error

type consumer struct {
	group   sarama.ConsumerGroup
	topics  []string
	handler messageHandler

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func createConsumer(brokers []string, groupID string, topics []string, handler messageHandler, options ...ConsumerOption) (Consumer, error) {
	cfg := &consumerBuildConfig{cfg: defaultSaramaConfig()}
	applyConsumerOptions(cfg, options)
	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg.cfg)
	if err != nil {
		return nil, err
	}
	return &consumer{
		group:   group,
		topics:  topics,
		handler: handler,
	}, nil
}

// NewConsumer creates a consumer for one topic with EventHandler[E] (JSON decode by default).
// Use WithHeaderKeys to pass selected headers into Handle; omit for no headers.
func NewConsumer[E any](brokers []string, groupID string, topic string, handler EventHandler[E], options ...ConsumerOption) (Consumer, error) {
	cfg := &consumerBuildConfig{cfg: defaultSaramaConfig()}
	applyConsumerOptions(cfg, options)
	adapted := adaptEventHandler[E](handler, cfg.headerKeys, withJSONDecoder[E]())
	return createConsumer(brokers, groupID, []string{topic}, adapted, options...)
}

func (c *consumer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.wg.Add(1)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for err := range c.group.Errors() {
			if err != nil {
				log.Printf("kafka consumer error: %v", err)
			}
		}
	}()
	go func() {
		defer c.wg.Done()
		for {
			if err := c.group.Consume(ctx, c.topics, &cgHandler{handler: c.handler}); err != nil {
				if errors.Is(err, sarama.ErrClosedConsumerGroup) {
					return
				}
				log.Printf("kafka consume error: %v", err)
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
	c.wg.Wait()
	return c.group.Close()
}

type cgHandler struct {
	handler messageHandler
}

func (h *cgHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *cgHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *cgHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := h.handler(sess.Context(), msg); err == nil {
			sess.MarkMessage(msg, "")
		}
	}
	return nil
}
