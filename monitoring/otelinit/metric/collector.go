package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
)

// WithGRPCMetricExporter allows you to send your metrics to the collector target
// collectorTarget is the address of the collector, e.g. "127.0.0.1:4317"
func WithGRPCMetricExporter(ctx context.Context, options ...otlpmetricgrpc.Option) ProviderOptionFunc {
	return func(pvd *provider) error {
		metricExporter, err := otlpmetricgrpc.New(ctx, options...)
		if err != nil {
			return fmt.Errorf("failed to create grpc metricExporter: %w", err)
		}

		pvd.metricExporter = metricExporter

		return nil
	}
}
