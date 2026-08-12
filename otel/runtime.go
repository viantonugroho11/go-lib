package otel

import (
	"context"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
)

// startRuntimeMetrics registers Go runtime instrumentation (goroutines, GC pause,
// heap alloc, memory stats) against the global meter provider. Returns a shutdown
// func that stops the collection goroutine.
//
// Metric names use the "go.*" namespace when the contrib package is at v0.55+ (semconv
// aligned): go.goroutine.count, go.memory.used, go.gc.pause, etc. Older versions still
// emit "process.runtime.go.*" — grep in Grafana to match your contrib version.
func startRuntimeMetrics() (func(context.Context) error, error) {
	if err := runtime.Start(runtime.WithMinimumReadMemStatsInterval(0)); err != nil {
		return nil, err
	}
	// contrib/runtime doesn't currently expose a Stop hook — it stops when the meter
	// provider shuts down. Return a no-op shutdown so Init's uniform lifecycle contract
	// stays satisfied.
	return func(context.Context) error { return nil }, nil
}
