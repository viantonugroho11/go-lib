## httpclient

Thin `http.Client` wrapper with base URL, retry, timeout, default headers, and context-based correlation propagation.

### Quick Start

```go
c := httpclient.New(
    httpclient.WithBaseURL("https://api.example.com"),
    httpclient.WithTimeout(10*time.Second),
    httpclient.WithRetry(3, 200*time.Millisecond),
    httpclient.WithHeader("X-Service", "payments"),
    httpclient.WithCorrelationHeader("request_id", "X-Request-ID"),
)

resp, err := c.Get(ctx, "/users/42", nil)
if err != nil { return err }
defer resp.Body.Close()
```

### Verbs

`Get`, `Post`, `Put`, `Patch`, `Delete` — all accept `ctx`, `path`, optional `body`, per-request `headers map[string]string`.

`Do(req *http.Request)` runs a fully-built request through the retry loop; `BaseURL` is NOT prepended.

### Retry

Only 5xx responses and network errors are retried. 4xx passes through unchanged. Body is buffered once so retries can rebuild the request.

### Options

- `WithBaseURL(url)` — prefix for `Get/Post/...` paths.
- `WithTimeout(d)` — per-request timeout (default 30s).
- `WithRetry(maxRetries, backoff)` — retry count + fixed backoff.
- `WithHeader(k, v)` — default header on every request.
- `WithTransport(rt)` — swap the underlying `http.RoundTripper` (e.g. `otelhttp.NewTransport(...)`).
- `WithCorrelationHeader(ctxKey, headerName)` — read `ctx.Value(ctxKey)` on each request and set it as `headerName`. Pairs with `httpserver.WithCorrelationHeader` for end-to-end tracing.

### End-to-End Correlation

`httpserver.WithCorrelationHeader("X-Trace-Id", myKey)` puts the header into ctx on ingress. Downstream `httpclient.WithCorrelationHeader(myKey, "X-Trace-Id")` reads it out and forwards it to the next hop. No handler changes.

### Benchmark

Apple M2 (arm64), against an in-process `httptest.Server`:

```
BenchmarkGet_NoRetry            ~41 μs/op       ~6.0 KB/op   ~73 allocs/op
BenchmarkGet_WithHeaders        ~44 μs/op       ~6.9 KB/op   ~85 allocs/op
BenchmarkPost_JSONBody          ~45 μs/op       ~8.1 KB/op   ~92 allocs/op
```

Numbers dominated by `net/http` server-side round-trip (localhost keep-alive); wrapper overhead ~1 μs.

### Example

See `example/` for a runnable client that hits a public JSON endpoint with retry and header defaults.
