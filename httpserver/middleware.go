package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"runtime/debug"
	"time"
)

// requestIDMiddleware reads or generates a request ID and stores it in ctx.
func requestIDMiddleware(header string, ctxKey any) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(header)
			if id == "" {
				id = newID()
			}
			w.Header().Set(header, id)
			ctx := context.WithValue(r.Context(), ctxKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// correlationMiddleware extracts inbound headers into ctx.
func correlationMiddleware(headers map[string]any) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			for h, k := range headers {
				if v := r.Header.Get(h); v != "" {
					ctx = context.WithValue(ctx, k, v)
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// recoverMiddleware turns panics into 500 with structured log.
func recoverMiddleware(logger LoggerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					if logger != nil {
						logger(r.Context(), r.Method, r.URL.Path, http.StatusInternalServerError, 0)
					}
					_ = rec
					_ = debug.Stack()
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// timeoutMiddleware wraps ctx with a deadline.
func timeoutMiddleware(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// loggingMiddleware calls logger after each request with method, path, status, duration.
func loggingMiddleware(logger LoggerFunc) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			logger(r.Context(), r.Method, r.URL.Path, sw.status, time.Since(start))
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusWriter) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(b)
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
