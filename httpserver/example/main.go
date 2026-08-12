// Example: chi-based server with request ID, correlation, timeouts, and health/ready.
//
// Try:
//
//	cd httpserver/example && go run .
//	curl -i http://localhost:8080/healthz
//	curl -i http://localhost:8080/users/42
//	curl -i -H 'X-Request-ID: my-id' http://localhost:8080/users/42
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/viantonugroho11/go-lib/httpserver"
)

type traceKey struct{}

func main() {
	srv := httpserver.New(
		httpserver.WithAddr(":8080"),
		httpserver.WithRequestTimeout(5*time.Second),
		httpserver.WithCorrelationHeader("X-Trace-Id", traceKey{}),
		httpserver.WithLogger(func(ctx context.Context, m, p string, s int, d time.Duration) {
			log.Printf("http %s %s -> %d in %s (request_id=%s trace_id=%v)",
				m, p, s, d.Truncate(time.Microsecond),
				httpserver.RequestIDFromContext(ctx), ctx.Value(traceKey{}))
		}),
		httpserver.WithReadyCheck(func(_ context.Context) error {
			// stand-in for db.Ping etc.
			return nil
		}),
	)

	srv.Router().Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		body := map[string]string{
			"id":         id,
			"request_id": httpserver.RequestIDFromContext(r.Context()),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	})

	srv.Router().Get("/boom", func(_ http.ResponseWriter, _ *http.Request) {
		panic("kaboom — the recover middleware turns this into 500")
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	fmt.Println("listening on :8080 — Ctrl+C to stop")
	if err := srv.Run(ctx); err != nil {
		log.Fatal(err)
	}
	_ = os.Stdout
}
