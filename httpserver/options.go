package httpserver

import (
	"context"
	"net/http"
	"time"
)

// Option configures the Server.
type Option func(*config)

// LoggerFunc logs a completed request. Wire to xlog.
type LoggerFunc func(ctx context.Context, method, path string, status int, dur time.Duration)

type config struct {
	addr             string
	readTimeout      time.Duration
	writeTimeout     time.Duration
	idleTimeout      time.Duration
	shutdownTimeout  time.Duration
	requestTimeout   time.Duration
	requestIDHeader  string
	requestIDCtxKey  any
	correlationHdrs  map[string]any // header name -> ctx key
	logger           LoggerFunc
	healthPath       string
	readyPath        string
	readyCheck       func(ctx context.Context) error
	disableRecover   bool
	extraMiddlewares []func(http.Handler) http.Handler
}

func defaultConfig() *config {
	return &config{
		addr:            ":8080",
		readTimeout:     10 * time.Second,
		writeTimeout:    30 * time.Second,
		idleTimeout:     60 * time.Second,
		shutdownTimeout: 15 * time.Second,
		requestTimeout:  0,
		requestIDHeader: "X-Request-ID",
		requestIDCtxKey: CtxKeyRequestID,
		correlationHdrs: make(map[string]any),
		healthPath:      "/healthz",
		readyPath:       "/readyz",
	}
}

// WithAddr sets listen address (default ":8080").
func WithAddr(addr string) Option {
	return func(c *config) { c.addr = addr }
}

// WithTimeouts sets read, write, idle timeouts on http.Server.
func WithTimeouts(read, write, idle time.Duration) Option {
	return func(c *config) {
		c.readTimeout = read
		c.writeTimeout = write
		c.idleTimeout = idle
	}
}

// WithShutdownTimeout bounds graceful shutdown (default 15s).
func WithShutdownTimeout(d time.Duration) Option {
	return func(c *config) { c.shutdownTimeout = d }
}

// WithRequestTimeout wraps each handler with a per-request context timeout.
// 0 disables (default).
func WithRequestTimeout(d time.Duration) Option {
	return func(c *config) { c.requestTimeout = d }
}

// WithRequestID overrides the request ID header name and context key.
// The middleware reads the header if present, else generates a new ID,
// stores it in ctx under ctxKey, and echoes it back in the response header.
func WithRequestID(header string, ctxKey any) Option {
	return func(c *config) {
		c.requestIDHeader = header
		c.requestIDCtxKey = ctxKey
	}
}

// WithCorrelationHeader extracts an inbound header into ctx under ctxKey.
// Pair with xlog.SetContextFieldExtractor to log the value automatically.
func WithCorrelationHeader(header string, ctxKey any) Option {
	return func(c *config) { c.correlationHdrs[header] = ctxKey }
}

// WithLogger wires a request logger (fired after each request completes).
func WithLogger(fn LoggerFunc) Option {
	return func(c *config) { c.logger = fn }
}

// WithHealthPath overrides "/healthz" (liveness). Empty disables.
func WithHealthPath(p string) Option {
	return func(c *config) { c.healthPath = p }
}

// WithReadyPath overrides "/readyz" (readiness). Empty disables.
func WithReadyPath(p string) Option {
	return func(c *config) { c.readyPath = p }
}

// WithReadyCheck sets the readiness probe. nil = always ready.
func WithReadyCheck(fn func(ctx context.Context) error) Option {
	return func(c *config) { c.readyCheck = fn }
}

// WithoutRecover disables the panic-recover middleware.
func WithoutRecover() Option {
	return func(c *config) { c.disableRecover = true }
}

// WithMiddleware appends a chi-compatible middleware, applied after built-ins.
func WithMiddleware(m func(http.Handler) http.Handler) Option {
	return func(c *config) { c.extraMiddlewares = append(c.extraMiddlewares, m) }
}
