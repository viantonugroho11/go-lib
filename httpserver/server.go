// Package httpserver provides an opinionated HTTP server built on chi
// with request ID, correlation propagation, panic recover, timeouts,
// structured logging hooks, and health/ready endpoints.
//
// Usage:
//
//	srv := httpserver.New(
//	    httpserver.WithAddr(":8080"),
//	    httpserver.WithLogger(func(ctx context.Context, m, p string, s int, d time.Duration) {
//	        xlog.Info(ctx, "http", xlog.String("method", m), xlog.Int("status", s))
//	    }),
//	    httpserver.WithReadyCheck(db.Ping),
//	)
//	srv.Router().Get("/users/{id}", getUser)
//	if err := srv.Run(ctx); err != nil { log.Fatal(err) }
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Server wraps *http.Server + chi.Router with graceful shutdown.
type Server struct {
	cfg    *config
	router chi.Router
	http   *http.Server
}

// New constructs a Server. Router() exposes the underlying chi.Router for route registration.
func New(opts ...Option) *Server {
	cfg := defaultConfig()
	for _, o := range opts {
		o(cfg)
	}

	r := chi.NewRouter()
	r.Use(requestIDMiddleware(cfg.requestIDHeader, cfg.requestIDCtxKey))
	if len(cfg.correlationHdrs) > 0 {
		r.Use(correlationMiddleware(cfg.correlationHdrs))
	}
	if !cfg.disableRecover {
		r.Use(recoverMiddleware(cfg.logger))
	}
	if cfg.requestTimeout > 0 {
		r.Use(timeoutMiddleware(cfg.requestTimeout))
	}
	if cfg.logger != nil {
		r.Use(loggingMiddleware(cfg.logger))
	}
	for _, m := range cfg.extraMiddlewares {
		r.Use(m)
	}

	if cfg.healthPath != "" {
		r.Get(cfg.healthPath, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})
	}
	if cfg.readyPath != "" {
		r.Get(cfg.readyPath, func(w http.ResponseWriter, req *http.Request) {
			if cfg.readyCheck != nil {
				if err := cfg.readyCheck(req.Context()); err != nil {
					http.Error(w, err.Error(), http.StatusServiceUnavailable)
					return
				}
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ready"))
		})
	}

	return &Server{
		cfg:    cfg,
		router: r,
		http: &http.Server{
			Addr:         cfg.addr,
			Handler:      r,
			ReadTimeout:  cfg.readTimeout,
			WriteTimeout: cfg.writeTimeout,
			IdleTimeout:  cfg.idleTimeout,
		},
	}
}

// Router returns the underlying chi.Router. Register routes before Run.
func (s *Server) Router() chi.Router { return s.router }

// Addr returns the configured listen address.
func (s *Server) Addr() string { return s.cfg.addr }

// Handler exposes the http.Handler (useful for httptest).
func (s *Server) Handler() http.Handler { return s.router }

// Run starts the server and blocks until ctx is cancelled, then shuts down gracefully.
// Returns nil on clean shutdown, or the underlying error otherwise.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("httpserver: listen: %w", err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.shutdownTimeout)
	defer cancel()
	if err := s.http.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("httpserver: shutdown: %w", err)
	}
	return nil
}

// Close forces the server to stop without waiting for active connections.
func (s *Server) Close() error { return s.http.Close() }
