package httpclient

import (
	"net/http"
	"time"
)

// Option configures the Client.
type Option func(*config)

type config struct {
	baseURL        string
	timeout        time.Duration
	maxRetries     int
	retryBackoff   time.Duration
	headers        map[string]string
	transport      http.RoundTripper
	correlationKey string // context key to propagate as a header
}

func defaultConfig() *config {
	return &config{
		timeout:      30 * time.Second,
		maxRetries:   0,
		retryBackoff: 100 * time.Millisecond,
		headers:      make(map[string]string),
	}
}

// WithBaseURL sets a base URL prepended to every request path.
func WithBaseURL(url string) Option {
	return func(c *config) { c.baseURL = url }
}

// WithTimeout sets the per-request timeout (default 30s).
func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithRetry sets max retry count and backoff between attempts.
// Only 5xx and network errors are retried.
func WithRetry(maxRetries int, backoff time.Duration) Option {
	return func(c *config) {
		c.maxRetries = maxRetries
		c.retryBackoff = backoff
	}
}

// WithHeader sets a default header on every request.
func WithHeader(key, value string) Option {
	return func(c *config) { c.headers[key] = value }
}

// WithTransport overrides the underlying http.RoundTripper.
func WithTransport(rt http.RoundTripper) Option {
	return func(c *config) { c.transport = rt }
}

// WithCorrelationHeader propagates ctx.Value(contextKey) as the given HTTP header on every request.
// Use together with xlog.SetContextFieldExtractor to forward trace/correlation IDs automatically.
func WithCorrelationHeader(contextKey, headerName string) Option {
	return func(c *config) {
		c.correlationKey = contextKey
		c.headers[headerName] = "" // placeholder; actual value filled per-request from ctx
	}
}
