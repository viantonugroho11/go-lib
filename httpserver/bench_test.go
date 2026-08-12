package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func BenchmarkServer_HealthEndpoint(b *testing.B) {
	srv := New(
		WithReadyCheck(func(_ context.Context) error { return nil }),
	)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
	}
}

func BenchmarkServer_UserHandler(b *testing.B) {
	srv := New(
		WithLogger(func(_ context.Context, _, _ string, _ int, _ time.Duration) {}),
	)
	srv.Router().Get("/users/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rr, req)
	}
}

func BenchmarkRequestIDMiddleware_Generate(b *testing.B) {
	mw := requestIDMiddleware("X-Request-ID", CtxKeyRequestID)
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
	handler := mw(next)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}

func BenchmarkRequestIDMiddleware_Passthru(b *testing.B) {
	mw := requestIDMiddleware("X-Request-ID", CtxKeyRequestID)
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
	handler := mw(next)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Request-ID", "provided-abcdef1234567890")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}
}
