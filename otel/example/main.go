// Example: bootstrap tracer + meter, emit one span + counter.
//
// This example points at localhost:4317 (OTLP gRPC). If no collector is running,
// exports fail silently in the background — the Init call still succeeds because
// gRPC is lazy. Set OTEL_EXPORTER_OTLP_ENDPOINT to your collector.
//
// Run:
//
//	cd otel/example && go run .
package main

import (
	"context"
	"log"
	"time"

	golibotel "github.com/viantonugroho11/go-lib/otel"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	shutdown, err := golibotel.Init(ctx,
		golibotel.WithServiceName("example-svc"),
		golibotel.WithServiceVersion("0.0.1"),
		golibotel.WithEnvironment("dev"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		sctx, scancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer scancel()
		_ = shutdown(sctx)
	}()

	tracer := golibotel.Tracer("example/main")
	meter := golibotel.Meter("example/main")
	counter, _ := meter.Int64Counter("example.requests")

	spanCtx, span := tracer.Start(ctx, "boot")
	counter.Add(spanCtx, 1)
	time.Sleep(100 * time.Millisecond)
	span.End()

	log.Println("done — check collector for span 'boot' and counter 'example.requests'")
}
