package otel

import (
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/contrib/propagators/jaeger"
	"go.opentelemetry.io/otel/propagation"
)

// PropagatorB3 returns a B3 propagator (single-header b3 format by default; use
// b3.WithInjectEncoding to change). Compatible with Zipkin, Istio sidecars,
// legacy services on B3.
//
//	otel.Init(ctx, otel.WithPropagators(
//	    propagation.TraceContext{},
//	    otel.PropagatorB3(),
//	))
func PropagatorB3() propagation.TextMapPropagator {
	return b3.New()
}

// PropagatorJaeger returns a Jaeger propagator (uber-trace-id header format).
// Compatible with legacy Jaeger-instrumented services.
func PropagatorJaeger() propagation.TextMapPropagator {
	return jaeger.Jaeger{}
}
