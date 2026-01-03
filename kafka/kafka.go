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

// MessageHandler menangani message; return error untuk tidak commit (akan diproses ulang).
type MessageHandler func(ctx context.Context, msg *sarama.ConsumerMessage) error

// TypedMessageHandler memproses event bertipe E yang sudah di-unmarshal.
// msg asli tetap diberikan jika butuh key/headers/metadata lain.
type TypedMessageHandler[E any] func(ctx context.Context, msg *sarama.ConsumerMessage, evt E) error

// HandlerOption mengonfigurasi cara inisialisasi dan decode event bertipe E.
type HandlerOption[E any] func(*handlerConfig[E])

type handlerConfig[E any] struct {
	newEvent func() E
	decode   func([]byte, *E) error
}

// WithNewEvent menyediakan konstruktor event bertipe E (untuk default values).
// Jika tidak diset, default-nya adalah new(E).
func WithNewEvent[E any](fn func() E) HandlerOption[E] {
	return func(c *handlerConfig[E]) { c.newEvent = fn }
}

// WithDecoder menyediakan fungsi decode khusus ke struct E.
// Contoh: JSON, Protobuf, Avro, dsb.
func WithDecoder[E any](fn func([]byte, *E) error) HandlerOption[E] {
	return func(c *handlerConfig[E]) { c.decode = fn }
}

// WithJSONDecoder mengatur decoder menjadi JSON (default jika tidak diset).
func WithJSONDecoder[E any]() HandlerOption[E] {
	return func(c *handlerConfig[E]) {
		c.decode = func(b []byte, dst *E) error { return json.Unmarshal(b, dst) }
	}
}

// AdaptTypedHandler membungkus TypedMessageHandler menjadi MessageHandler biasa.
// - Inisialisasi struct E dilakukan via WithNewEvent (opsional).
// - Unmarshal dilakukan otomatis via decoder (default JSON jika tidak diset).
func AdaptTypedHandler[E any](th TypedMessageHandler[E], opts ...HandlerOption[E]) MessageHandler {
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

type Consumer struct {
	group   sarama.ConsumerGroup
	topics  []string
	handler MessageHandler

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ConsumerOption untuk mengonfigurasi sarama.Config sebelum membuat consumer group.
type ConsumerOption func(cfg *sarama.Config)

// NewConsumer membuat consumer group generik untuk satu atau banyak topik.
func NewConsumer(brokers []string, groupID string, topics []string, handler MessageHandler, options ...ConsumerOption) (*Consumer, error) {
	cfg := sarama.NewConfig()
	// Defaults aman dan umum
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

	for _, opt := range options {
		opt(cfg)
	}

	group, err := sarama.NewConsumerGroup(brokers, groupID, cfg)
	if err != nil {
		return nil, err
	}
	return &Consumer{
		group:   group,
		topics:  topics,
		handler: handler,
	}, nil
}

func (c *Consumer) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.wg.Add(1)
	// Drain error channel agar tidak deadlock
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

func (c *Consumer) Close() error {
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
			// Commit hanya jika handler sukses
			sess.MarkMessage(msg, "")
		}
	}
	return nil
}

// ---------- Opsi umum ----------

// WithConsumerClientID set client id
func WithConsumerClientID(clientID string) ConsumerOption {
	return func(cfg *sarama.Config) { cfg.ClientID = clientID }
}

// WithConsumerVersion set Kafka version
func WithConsumerVersion(version sarama.KafkaVersion) ConsumerOption {
	return func(cfg *sarama.Config) { cfg.Version = version }
}

// WithInitialOffset pilih offset awal (Newest/Oldest)
func WithInitialOffset(offset int64) ConsumerOption {
	return func(cfg *sarama.Config) { cfg.Consumer.Offsets.Initial = offset }
}

// WithRebalanceStrategy pilih strategi rebalance
func WithRebalanceStrategy(strategy sarama.BalanceStrategy) ConsumerOption {
	return func(cfg *sarama.Config) { cfg.Consumer.Group.Rebalance.Strategy = strategy }
}

// WithGroupSessionTimeout set session timeout
func WithGroupSessionTimeout(d time.Duration) ConsumerOption {
	return func(cfg *sarama.Config) { cfg.Consumer.Group.Session.Timeout = d }
}

// WithGroupHeartbeatInterval set heartbeat interval
func WithGroupHeartbeatInterval(d time.Duration) ConsumerOption {
	return func(cfg *sarama.Config) { cfg.Consumer.Group.Heartbeat.Interval = d }
}

// WithNetTimeouts set dial/read/write timeout
func WithNetTimeouts(dial, read, write time.Duration) ConsumerOption {
	return func(cfg *sarama.Config) {
		cfg.Net.DialTimeout = dial
		cfg.Net.ReadTimeout = read
		cfg.Net.WriteTimeout = write
	}
}

// WithTLSEnable aktifkan TLS; jika insecure true maka skip verifikasi sertifikat
func WithTLSEnable(insecureSkipVerify bool) ConsumerOption {
	return func(cfg *sarama.Config) {
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config = &tls.Config{InsecureSkipVerify: insecureSkipVerify} //nolint:gosec
	}
}

// WithSASLPlain aktifkan SASL PLAIN
func WithSASLPlain(username, password string) ConsumerOption {
	return func(cfg *sarama.Config) {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.User = username
		cfg.Net.SASL.Password = password
		cfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}
}

// ---------- ENV helper ----------

// NewConsumerFromEnv membuat consumer dari environment variables (prefixable).
// Contoh variabel (prefix "KAFKA_"):
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
func NewConsumerFromEnv(brokersEnvPrefix string, groupID string, topics []string, handler MessageHandler, overrides ...ConsumerOption) (*Consumer, error) {
	brokersStr := strings.TrimSpace(os.Getenv(brokersEnvPrefix + "BROKERS"))
	if brokersStr == "" {
		return nil, errors.New("missing " + brokersEnvPrefix + "BROKERS")
	}
	brokers := splitAndTrim(brokersStr)

	opts := make([]ConsumerOption, 0, 8)

	if v := strings.TrimSpace(os.Getenv(brokersEnvPrefix + "CLIENT_ID")); v != "" {
		opts = append(opts, WithConsumerClientID(v))
	}
	if v := strings.TrimSpace(os.Getenv(brokersEnvPrefix + "VERSION")); v != "" {
		if ver, err := sarama.ParseKafkaVersion(v); err == nil {
			opts = append(opts, WithConsumerVersion(ver))
		}
	}
	if v := strings.TrimSpace(os.Getenv(brokersEnvPrefix + "OFFSET_INITIAL")); v != "" {
		switch strings.ToLower(v) {
		case "oldest":
			opts = append(opts, WithInitialOffset(sarama.OffsetOldest))
		default:
			opts = append(opts, WithInitialOffset(sarama.OffsetNewest))
		}
	}
	if v := strings.TrimSpace(os.Getenv(brokersEnvPrefix + "REBALANCE_STRATEGY")); v != "" {
		switch strings.ToLower(v) {
		case "round_robin":
			opts = append(opts, WithRebalanceStrategy(sarama.BalanceStrategyRoundRobin))
		case "sticky":
			opts = append(opts, WithRebalanceStrategy(sarama.BalanceStrategySticky))
		default:
			opts = append(opts, WithRebalanceStrategy(sarama.BalanceStrategyRange))
		}
	}
	// TLS
	if b := parseBool(os.Getenv(brokersEnvPrefix + "TLS_ENABLE")); b {
		insecure := parseBool(os.Getenv(brokersEnvPrefix + "TLS_INSECURE_SKIP_VERIFY"))
		opts = append(opts, WithTLSEnable(insecure))
	}
	// SASL (PLAIN only)
	if b := parseBool(os.Getenv(brokersEnvPrefix + "SASL_ENABLE")); b {
		mech := strings.ToUpper(strings.TrimSpace(os.Getenv(brokersEnvPrefix + "SASL_MECHANISM")))
		user := os.Getenv(brokersEnvPrefix + "SASL_USERNAME")
		pass := os.Getenv(brokersEnvPrefix + "SASL_PASSWORD")
		if mech == "" || mech == "PLAIN" {
			opts = append(opts, WithSASLPlain(user, pass))
		} else {
			return nil, errors.New("unsupported SASL mechanism: " + mech + " (only PLAIN supported)")
		}
	}
	// overrides terakhir
	opts = append(opts, overrides...)

	return NewConsumer(brokers, groupID, topics, handler, opts...)
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
