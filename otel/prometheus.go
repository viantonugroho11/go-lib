package otel

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// newPrometheusReader builds a Prometheus exporter/reader and registers the scrape
// handler on cfg.promMux at cfg.promPath. Returns the reader for MeterProvider wiring.
func newPrometheusReader(cfg *config) (sdkmetric.Reader, error) {
	reader, err := prometheus.New()
	if err != nil {
		return nil, err
	}
	cfg.promMux.Handle(cfg.promPath, promhttp.Handler())
	return reader, nil
}
