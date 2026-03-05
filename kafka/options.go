package kafka

import (
	"crypto/tls"
	"time"

	"github.com/IBM/sarama"
	"github.com/rcrowley/go-metrics"
)

type consumerBuildConfig struct {
	cfg        *sarama.Config
	headerKeys []string
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

// WithHeaderKeys sets which header keys are passed to EventHandler.Handle. Omit to pass no headers.
func WithHeaderKeys(keys ...string) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.headerKeys = keys }}
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

// clientID sets the client ID.
func WithClientID(clientID string) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.ClientID = clientID }}
}

// version sets the Kafka version.
func WithVersion(version sarama.KafkaVersion) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Version = version }}
}

// returnErrors sets the return errors flag.
func WithReturnErrors(enable bool) ConsumerOption {
	return &consumerOptionFunc{fn: func(c *consumerBuildConfig) { c.cfg.Consumer.Return.Errors = enable }}
}
// returnErrors sets the return errors flag.


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