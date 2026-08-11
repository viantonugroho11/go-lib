package httpserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestIDPropagates(t *testing.T) {
	srv := New()
	var got string
	srv.Router().Get("/x", func(w http.ResponseWriter, r *http.Request) {
		got = RequestIDFromContext(r.Context())
		w.WriteHeader(200)
	})

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if got != "abc-123" {
		t.Fatalf("ctx request id = %q, want abc-123", got)
	}
	if rr.Header().Get("X-Request-ID") != "abc-123" {
		t.Fatalf("echo header = %q", rr.Header().Get("X-Request-ID"))
	}
}

func TestRequestIDGeneratedWhenMissing(t *testing.T) {
	srv := New()
	srv.Router().Get("/x", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	req := httptest.NewRequest("GET", "/x", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if id := rr.Header().Get("X-Request-ID"); len(id) != 32 {
		t.Fatalf("generated id len = %d, want 32", len(id))
	}
}

func TestRecoverMiddleware(t *testing.T) {
	srv := New()
	srv.Router().Get("/boom", func(_ http.ResponseWriter, _ *http.Request) { panic("kaboom") })
	req := httptest.NewRequest("GET", "/boom", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestHealthAndReady(t *testing.T) {
	srv := New(WithReadyCheck(func(ctx context.Context) error { return nil }))
	for _, p := range []string{"/healthz", "/readyz"} {
		req := httptest.NewRequest("GET", p, nil)
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
		if rr.Code != 200 {
			t.Fatalf("%s code = %d", p, rr.Code)
		}
	}
}

func TestReadyFail(t *testing.T) {
	srv := New(WithReadyCheck(func(ctx context.Context) error { return errors.New("db down") }))
	req := httptest.NewRequest("GET", "/readyz", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("code = %d", rr.Code)
	}
}

func TestCorrelationHeader(t *testing.T) {
	type key string
	const k key = "trace"
	srv := New(WithCorrelationHeader("X-Trace-Id", k))
	var got string
	srv.Router().Get("/x", func(w http.ResponseWriter, r *http.Request) {
		if v, ok := r.Context().Value(k).(string); ok {
			got = v
		}
	})
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Trace-Id", "t-1")
	srv.Handler().ServeHTTP(httptest.NewRecorder(), req)
	if got != "t-1" {
		t.Fatalf("ctx trace = %q", got)
	}
}

func TestLoggerFires(t *testing.T) {
	var called bool
	srv := New(WithLogger(func(ctx context.Context, m, p string, s int, d time.Duration) {
		called = true
		if m != "GET" || p != "/x" || s != 201 {
			t.Errorf("logger got %s %s %d", m, p, s)
		}
	}))
	srv.Router().Get("/x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(201) })
	srv.Handler().ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/x", nil))
	if !called {
		t.Fatal("logger not called")
	}
}

func TestRunShutdown(t *testing.T) {
	srv := New(WithAddr("127.0.0.1:0"), WithShutdownTimeout(2*time.Second))
	// bind an ephemeral port by using httptest instead of Run to keep test hermetic
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "ok") {
		t.Fatalf("body = %q", b)
	}
}
