package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

// Consumer is the interface for a Kafka consumer group. Implementations support Start and Close.
// Returned by NewConsumer, NewTypedConsumer, NewConsumerWithHandler, and their FromEnv variants.
type Consumer interface {
	Start(ctx context.Context)
	Close() error
}

// MessageHandler handles a single message; return error to avoid commit (message will be retried).
type MessageHandler func(ctx context.Context, msg *sarama.ConsumerMessage) error

// TypedMessageHandler processes a typed event E. Used by NewTypedConsumer.
type TypedMessageHandler[E any] func(ctx context.Context, msg *sarama.ConsumerMessage, evt E) error

// handlerOption configures decode/init for typed handlers (internal).
type handlerOption[E any] func(*handlerConfig[E])

type handlerConfig[E any] struct {
	newEvent func() E
	decode   func([]byte, *E) error
}

func withNewEvent[E any](fn func() E) handlerOption[E] {
	return func(c *handlerConfig[E]) { c.newEvent = fn }
}

func withDecoder[E any](fn func([]byte, *E) error) handlerOption[E] {
	return func(c *handlerConfig[E]) { c.decode = fn }
}

func withJSONDecoder[E any]() handlerOption[E] {
	return func(c *handlerConfig[E]) {
		c.decode = func(b []byte, dst *E) error { return json.Unmarshal(b, dst) }
	}
}

func adaptTypedHandler[E any](th TypedMessageHandler[E], opts ...handlerOption[E]) MessageHandler {
	cfg := &handlerConfig[E]{
		newEvent: func() E { var zero E; return zero },
		decode:   func(b []byte, dst *E) error { return json.Unmarshal(b, dst) },
	}
	for _, o := range opts {
		o(cfg)
	}
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		evt := cfg.newEvent()
		if err := cfg.decode(msg.Value, &evt); err != nil {
			return err
		}
		return th(ctx, msg, evt)
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

func adaptEventHandler[E any](handler EventHandler[E], headerKeys []string, opts ...handlerOption[E]) MessageHandler {
	cfg := &handlerConfig[E]{
		newEvent: func() E { var zero E; return zero },
		decode:   func(b []byte, dst *E) error { return json.Unmarshal(b, dst) },
	}
	for _, o := range opts {
		o(cfg)
	}
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		evt := cfg.newEvent()
		if err := cfg.decode(msg.Value, &evt); err != nil {
			return err
		}
		all := headersFromMessage(msg)
		headers := filterHeadersByKeys(all, headerKeys)
		progress := handler.Handle(ctx, evt, headers...)
		if progress.Status == ProgressError {
			if progress.Err != nil {
				return progress.Err
			}
			return errors.New(progress.Result)
		}
		return nil
	}
}

type consumer struct {
	group   sarama.ConsumerGroup
	topics  []string
	handler MessageHandler

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// consumerBuildConfig dipakai saat apply ConsumerOption (sarama config + header keys untuk handler).
type consumerBuildConfig struct {
	cfg        *sarama.Config
	headerKeys []string
}

// ConsumerOption configures consumer group (sarama) and/or header keys for EventHandler.
// Use WithHeaderKeys to pass only certain header keys into Handle; omit to pass no headers.
type ConsumerOption interface {
	apply(*consumerBuildConfig)
}

type consumerOptionFunc struct{ fn func(*consumerBuildConfig) }

func (o *consumerOptionFunc) apply(c *consumerBuildConfig) { o.fn(c) }

func defaultSaramaConfig() *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.ClientID = "go-lib-kafka"
	cfg.Version = sarama.V2_8_0_0
	cfg.Consumer.Return.Errors = true
	cfg.Consumer.Group.Heartbeat.Interval = 3 * time.Second
	cfg.Consumer.Group.Session.Timeout = 30 * time.Second
	cfg.Consumer.Group.Rebalance.Timeout = 30 * time.Second
	cfg.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
	cfg.Consumer.Offsets.AutoCommit.Enable = true
	cfg.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second
	cfg.Consumer.Offsets.Initial = sarama.OffsetNewest
	return cfg
}

func applyConsumerOptions(c *consumerBuildConfig, options []ConsumerOption) {
	for _, opt := range options {
		opt.apply(c)
	}
}

// NewConsumer creates a consumer group for one or more topics. Returns the Consumer interface.
func NewConsumer(brokers []string, groupID string, topics []string, handler MessageHandler, options ...ConsumerOption) (Consumer, error) {
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

// NewTypedConsumer creates a consumer for one topic with a typed handler (JSON decode by default).
func NewTypedConsumer[E any](brokers []string, groupID string, topic string, handler TypedMessageHandler[E], options ...ConsumerOption) (Consumer, error) {
	adapted := adaptTypedHandler[E](handler, withJSONDecoder[E]())
	return NewConsumer(brokers, groupID, []string{topic}, adapted, options...)
}

// WithHeaderKeys sets which header keys are passed to EventHandler.Handle. Omit to pass no headers.
func WithHeaderKeys(keys ...string) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.headerKeys = keys }}
}

// NewConsumerWithHandler creates a consumer with EventHandler[E] (JSON decode by default).
func NewConsumerWithHandler[E any](brokers []string, groupID string, topic string, handler EventHandler[E], options ...ConsumerOption) (Consumer, error) {
	cfg := &consumerBuildConfig{cfg: defaultSaramaConfig()}
	applyConsumerOptions(cfg, options)
	adapted := adaptEventHandler[E](handler, cfg.headerKeys, withJSONDecoder[E]())
	return NewConsumer(brokers, groupID, []string{topic}, adapted, options...)
}

// NewConsumerWithHandlerFromEnv like NewConsumerWithHandler but reads brokers and options from env (e.g. KAFKA_BROKERS).
func NewConsumerWithHandlerFromEnv[E any](envPrefix string, groupID string, topic string, handler EventHandler[E], overrides ...ConsumerOption) (Consumer, error) {
	brokersStr := strings.TrimSpace(os.Getenv(envPrefix + "BROKERS"))
	if brokersStr == "" {
		return nil, errors.New("missing " + envPrefix + "BROKERS")
	}
	brokers := splitAndTrim(brokersStr)
	opts := envToConsumerOptions(envPrefix)
	opts = append(opts, overrides...)
	cfg := &consumerBuildConfig{cfg: defaultSaramaConfig()}
	applyConsumerOptions(cfg, opts)
	adapted := adaptEventHandler[E](handler, cfg.headerKeys, withJSONDecoder[E]())
	return NewConsumer(brokers, groupID, []string{topic}, adapted, opts...)
}

func (c *consumer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.wg.Add(1)
	// Drain the error channel to avoid deadlocks
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
	handler MessageHandler
}

func (h *cgHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *cgHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (h *cgHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		if err := h.handler(sess.Context(), msg); err == nil {
			// Commit only when the handler succeeds
			sess.MarkMessage(msg, "")
		}
	}
	return nil
}

// ---------- Opsi umum ----------

// WithConsumerClientID sets the client id.
func WithConsumerClientID(clientID string) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.ClientID = clientID }}
}

// WithConsumerVersion sets the Kafka version.
func WithConsumerVersion(version sarama.KafkaVersion) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Version = version }}
}

// WithInitialOffset chooses the initial offset (Newest/Oldest).
func WithInitialOffset(offset int64) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Consumer.Offsets.Initial = offset }}
}

// WithRebalanceStrategy chooses the rebalance strategy.
func WithRebalanceStrategy(strategy sarama.BalanceStrategy) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Consumer.Group.Rebalance.Strategy = strategy }}
}

// WithGroupSessionTimeout sets the session timeout.
func WithGroupSessionTimeout(d time.Duration) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Consumer.Group.Session.Timeout = d }}
}

// WithGroupHeartbeatInterval sets the heartbeat interval.
func WithGroupHeartbeatInterval(d time.Duration) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Consumer.Group.Heartbeat.Interval = d }}
}

// WithNetTimeouts sets dial/read/write timeouts.
func WithNetTimeouts(dial, read, write time.Duration) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		c.cfg.Net.DialTimeout = dial
		c.cfg.Net.ReadTimeout = read
		c.cfg.Net.WriteTimeout = write
	}}
}

// WithTLSEnable enables TLS; if insecureSkipVerify is true, certificate verification is skipped.
func WithTLSEnable(insecureSkipVerify bool) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		c.cfg.Net.TLS.Enable = true
		c.cfg.Net.TLS.Config = &tls.Config{InsecureSkipVerify: insecureSkipVerify} //nolint:gosec
	}}
}

// WithSASLPlain enables SASL PLAIN.
func WithSASLPlain(username, password string) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		c.cfg.Net.SASL.Enable = true
		c.cfg.Net.SASL.User = username
		c.cfg.Net.SASL.Password = password
		c.cfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}}
}

// ---------- ENV helper ----------

// NewConsumerFromEnv creates a consumer from environment variables (prefixable).
// Example variables (prefix "KAFKA_"):
// - KAFKA_BROKERS=host1:9092,host2:9092
// - KAFKA_CLIENT_ID=my-app
// - KAFKA_VERSION=2.8.0
// - KAFKA_OFFSET_INITIAL=newest|oldest
// - KAFKA_REBALANCE_STRATEGY=range|round_robin|sticky
// - KAFKA_TLS_ENABLE=true|false
// - KAFKA_TLS_INSECURE_SKIP_VERIFY=true|false
// - KAFKA_SASL_ENABLE=true|false
// - KAFKA_SASL_MECHANISM=PLAIN
// - KAFKA_SASL_USERNAME=user
// - KAFKA_SASL_PASSWORD=pass
func envToConsumerOptions(envPrefix string) []ConsumerOption {
	opts := make([]ConsumerOption, 0, 8)
	if v := strings.TrimSpace(os.Getenv(envPrefix + "CLIENT_ID")); v != "" {
		opts = append(opts, WithConsumerClientID(v))
	}
	if v := strings.TrimSpace(os.Getenv(envPrefix + "VERSION")); v != "" {
		if ver, err := sarama.ParseKafkaVersion(v); err == nil {
			opts = append(opts, WithConsumerVersion(ver))
		}
	}
	if v := strings.TrimSpace(os.Getenv(envPrefix + "OFFSET_INITIAL")); v != "" {
		switch strings.ToLower(v) {
		case "oldest":
			opts = append(opts, WithInitialOffset(sarama.OffsetOldest))
		default:
			opts = append(opts, WithInitialOffset(sarama.OffsetNewest))
		}
	}
	if v := strings.TrimSpace(os.Getenv(envPrefix + "REBALANCE_STRATEGY")); v != "" {
		switch strings.ToLower(v) {
		case "round_robin":
			opts = append(opts, WithRebalanceStrategy(sarama.BalanceStrategyRoundRobin))
		case "sticky":
			opts = append(opts, WithRebalanceStrategy(sarama.BalanceStrategySticky))
		default:
			opts = append(opts, WithRebalanceStrategy(sarama.BalanceStrategyRange))
		}
	}
	if b := parseBool(os.Getenv(envPrefix + "TLS_ENABLE")); b {
		insecure := parseBool(os.Getenv(envPrefix + "TLS_INSECURE_SKIP_VERIFY"))
		opts = append(opts, WithTLSEnable(insecure))
	}
	if b := parseBool(os.Getenv(envPrefix + "SASL_ENABLE")); b {
		mech := strings.ToUpper(strings.TrimSpace(os.Getenv(envPrefix + "SASL_MECHANISM")))
		user := os.Getenv(envPrefix + "SASL_USERNAME")
		pass := os.Getenv(envPrefix + "SASL_PASSWORD")
		if mech == "" || mech == "PLAIN" {
			opts = append(opts, WithSASLPlain(user, pass))
		}
	}
	return opts
}

// NewConsumerFromEnv creates a consumer from env vars (e.g. KAFKA_BROKERS, KAFKA_CLIENT_ID). See envToConsumerOptions.
func NewConsumerFromEnv(brokersEnvPrefix string, groupID string, topics []string, handler MessageHandler, overrides ...ConsumerOption) (Consumer, error) {
	brokersStr := strings.TrimSpace(os.Getenv(brokersEnvPrefix + "BROKERS"))
	if brokersStr == "" {
		return nil, errors.New("missing " + brokersEnvPrefix + "BROKERS")
	}
	brokers := splitAndTrim(brokersStr)
	opts := append(envToConsumerOptions(brokersEnvPrefix), overrides...)
	return NewConsumer(brokers, groupID, topics, handler, opts...)
}

// NewTypedConsumerFromEnv like NewConsumerFromEnv for one topic with TypedMessageHandler[E].
func NewTypedConsumerFromEnv[E any](envPrefix string, groupID string, topic string, handler TypedMessageHandler[E], overrides ...ConsumerOption) (Consumer, error) {
	adapted := adaptTypedHandler[E](handler, withJSONDecoder[E]())
	return NewConsumerFromEnv(envPrefix, groupID, []string{topic}, adapted, overrides...)
}

// helpers
func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p) 
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseBool(s string) bool {
	if s == "" {
		return false
	}
	b, err := strconv.ParseBool(s)
	return err == nil && b
}
