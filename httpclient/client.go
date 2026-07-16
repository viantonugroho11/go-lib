// Package httpclient provides a thin http.Client wrapper with retry, timeout,
// default headers, and context-based correlation ID propagation.
//
// Usage:
//
//	c := httpclient.New(
//	    httpclient.WithBaseURL("https://api.example.com"),
//	    httpclient.WithTimeout(10*time.Second),
//	    httpclient.WithRetry(3, 200*time.Millisecond),
//	    httpclient.WithHeader("X-Service", "my-service"),
//	)
//	resp, err := c.Get(ctx, "/users/1", nil)
package httpclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps http.Client with retry, default headers, and correlation propagation.
type Client struct {
	cfg    *config
	client *http.Client
}

// New creates a Client with the given options.
func New(opts ...Option) *Client {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}
	transport := cfg.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &Client{
		cfg: cfg,
		client: &http.Client{
			Timeout:   cfg.timeout,
			Transport: transport,
		},
	}
}

// Get sends a GET request. headers are per-request overrides.
func (c *Client) Get(ctx context.Context, path string, headers map[string]string) (*http.Response, error) {
	return c.do(ctx, http.MethodGet, path, nil, headers)
}

// Post sends a POST request with body.
func (c *Client) Post(ctx context.Context, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	return c.do(ctx, http.MethodPost, path, body, headers)
}

// Put sends a PUT request with body.
func (c *Client) Put(ctx context.Context, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	return c.do(ctx, http.MethodPut, path, body, headers)
}

// Patch sends a PATCH request with body.
func (c *Client) Patch(ctx context.Context, path string, body io.Reader, headers map[string]string) (*http.Response, error) {
	return c.do(ctx, http.MethodPatch, path, body, headers)
}

// Delete sends a DELETE request.
func (c *Client) Delete(ctx context.Context, path string, headers map[string]string) (*http.Response, error) {
	return c.do(ctx, http.MethodDelete, path, nil, headers)
}

// Do sends a fully-configured *http.Request through retry logic.
// BaseURL is NOT prepended when using Do directly.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.doWithRetry(req.Context(), req, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, extraHeaders map[string]string) (*http.Response, error) {
	url := c.cfg.baseURL + path

	// Buffer body for retries.
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = io.ReadAll(body)
		if err != nil {
			return nil, fmt.Errorf("httpclient: read body: %w", err)
		}
	}

	newReq := func() (*http.Request, error) {
		var r io.Reader
		if bodyBytes != nil {
			r = bytes.NewReader(bodyBytes)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, r)
		if err != nil {
			return nil, err
		}
		// apply default headers
		for k, v := range c.cfg.headers {
			req.Header.Set(k, v)
		}
		// propagate correlation ID from context
		if c.cfg.correlationKey != "" {
			if v, ok := ctx.Value(c.cfg.correlationKey).(string); ok && v != "" {
				// header name was stored as the placeholder key in c.cfg.headers,
				// but we need the actual header name; use correlationKey as header name
				// unless caller set a specific header name via WithCorrelationHeader.
				req.Header.Set(c.cfg.correlationKey, v)
			}
		}
		// per-request overrides
		for k, v := range extraHeaders {
			req.Header.Set(k, v)
		}
		return req, nil
	}

	req, err := newReq()
	if err != nil {
		return nil, fmt.Errorf("httpclient: build request: %w", err)
	}
	return c.doWithRetry(ctx, req, newReq)
}

func (c *Client) doWithRetry(ctx context.Context, req *http.Request, newReq func() (*http.Request, error)) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)
	attempts := c.cfg.maxRetries + 1
	for i := range attempts {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(c.cfg.retryBackoff):
			}
			if newReq != nil {
				req, err = newReq()
				if err != nil {
					return nil, fmt.Errorf("httpclient: rebuild request: %w", err)
				}
			}
		}
		resp, err = c.client.Do(req)
		if err != nil {
			// network error — retry
			continue
		}
		if resp.StatusCode < 500 {
			// 1xx-4xx: do not retry
			return resp, nil
		}
		// 5xx: close body and retry
		_ = resp.Body.Close()
	}
	if err != nil {
		return nil, fmt.Errorf("httpclient: %s %s after %d attempt(s): %w", req.Method, req.URL, attempts, err)
	}
	return resp, nil
}
