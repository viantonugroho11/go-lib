package kafka

import (
	"crypto/tls"
	"time"

	"github.com/IBM/sarama"
	"github.com/rcrowley/go-metrics"
)

type consumerBuildConfig struct {
	cfg         *sarama.Config
	headerKeys  []string
	logger      Logger
	strategy    ErrorStrategy
	dlq         DeadLetterFunc
	backoff     BlockBackoff
	concurrency int
}

// ConsumerOption configures consumer group (sarama) and/or header keys for EventHandler.
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
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
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

// WithHeaderKeys sets which header keys are passed to EventHandler.Handle. Omit to pass no headers.
func WithHeaderKeys(keys ...string) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.headerKeys = keys }}
}

// WithLogger overrides the default stderr logger. Wire xlog or slog wrapper here.
func WithLogger(l Logger) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		if l != nil {
			c.logger = l
		}
	}}
}

// WithErrorStrategy sets how the consumer reacts to ProgressError from the handler.
// ErrorSkip (default) advances offset, ErrorBlock retries until success, ErrorDeadLetter routes to DLQ.
func WithErrorStrategy(s ErrorStrategy) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.strategy = s }}
}

// WithDeadLetter wires a DLQ publish function; setting this also enables ErrorDeadLetter unless overridden.
func WithDeadLetter(fn DeadLetterFunc) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		c.dlq = fn
		if c.strategy == ErrorSkip {
			c.strategy = ErrorDeadLetter
		}
	}}
}

// WithConcurrencyPerPartition sets the number of worker goroutines per partition claim (default 1).
// Values > 1 process messages concurrently within a claim, then commit the highest CONTIGUOUS
// completed offset. Increases throughput for I/O-bound handlers but breaks per-key ordering —
// use only when handlers are commutative or ordering is enforced elsewhere.
func WithConcurrencyPerPartition(n int) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		if n < 1 {
			n = 1
		}
		c.concurrency = n
	}}
}

// WithBlockOnError is shorthand for WithErrorStrategy(ErrorBlock) with a custom backoff.
// Pass zero BlockBackoff to keep defaults (100ms initial, 30s max, x2 factor).
func WithBlockOnError(b BlockBackoff) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		c.strategy = ErrorBlock
		if b.Initial > 0 {
			c.backoff = b
		}
	}}
}

// WithConsumerClientID sets the client id.
func WithConsumerClientID(clientID string) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.ClientID = clientID }}
}

// WithConsumerVersion sets the Kafka version.
func WithConsumerVersion(version sarama.KafkaVersion) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Version = version }}
}

// WithInitialOffset sets initial offset (e.g. sarama.OffsetNewest, sarama.OffsetOldest).
func WithInitialOffset(offset int64) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Consumer.Offsets.Initial = offset }}
}

// WithRebalanceStrategy sets the rebalance strategy.
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

// WithTLSEnable enables TLS. Set insecureSkipVerify to skip certificate verification.
func WithTLSEnable(insecureSkipVerify bool) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		c.cfg.Net.TLS.Enable = true
		c.cfg.Net.TLS.Config = &tls.Config{InsecureSkipVerify: insecureSkipVerify} //nolint:gosec
	}}
}

// WithSASLPlain enables SASL PLAIN authentication.
func WithSASLPlain(username, password string) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) {
		c.cfg.Net.SASL.Enable = true
		c.cfg.Net.SASL.User = username
		c.cfg.Net.SASL.Password = password
		c.cfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	}}
}

// WithReturnErrors sets the return errors flag.
func WithReturnErrors(enable bool) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Consumer.Return.Errors = enable }}
}

// channelBufferSize sets the channel buffer size.
func WithChannelBufferSize(n int) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.ChannelBufferSize = n }}
}


// apiVersionsRequest sets the API versions request flag.
func WithApiVersionsRequest(enable bool) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.ApiVersionsRequest = enable }}
}

// WithMetricRegistry sets the metric registry (sarama uses github.com/rcrowley/go-metrics).
func WithMetricRegistry(registry metrics.Registry) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.MetricRegistry = registry }}
}

// metricRegistry sets the metric registry.