package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

// One shared server + shared transport across all httpclient benches. Without this,
// each bench spins a fresh Server + Transport, and macOS's ephemeral port pool + TIME_WAIT
// starves later benches ("can't assign requested address"). Reusing the transport keeps
// idle connections and eliminates the fresh-dial-per-iteration problem.
var (
	benchOnce   sync.Once
	benchServer *httptest.Server
	benchTx     http.RoundTripper
)

func benchSetup() {
	benchOnce.Do(func() {
		benchServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}))
		tx := &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			MaxConnsPerHost:     100,
		}
		benchTx = tx
	})
}

func TestMain(m *testing.M) {
	code := m.Run()
	if benchServer != nil {
		benchServer.Close()
	}
	os.Exit(code)
}

// drain closes body after fully reading it so keep-alive can reuse the connection.
func drain(resp *http.Response) {
	if resp == nil {
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
}

func BenchmarkGet_NoRetry(b *testing.B) {
	benchSetup()
	c := New(WithBaseURL(benchServer.URL), WithTransport(benchTx))
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := c.Get(ctx, "/x", nil)
		if err != nil {
			b.Fatal(err)
		}
		drain(resp)
	}
}

func BenchmarkGet_WithHeaders(b *testing.B) {
	benchSetup()
	c := New(
		WithBaseURL(benchServer.URL),
		WithTransport(benchTx),
		WithHeader("X-Service", "bench"),
		WithHeader("X-Region", "sea"),
	)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := c.Get(ctx, "/x", map[string]string{"X-Trace-Id": "t-1"})
		if err != nil {
			b.Fatal(err)
		}
		drain(resp)
	}
}

func BenchmarkPost_JSONBody(b *testing.B) {
	benchSetup()
	c := New(WithBaseURL(benchServer.URL), WithTransport(benchTx), WithHeader("Content-Type", "application/json"))
	payload := []byte(`{"id":42,"note":"benchmark body payload"}`)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := c.Post(ctx, "/x", bytes.NewReader(payload), nil)
		if err != nil {
			b.Fatal(err)
		}
		drain(resp)
	}
}
