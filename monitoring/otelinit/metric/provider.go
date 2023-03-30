package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

// ProviderOptionFunc is the option function for provider
type ProviderOptionFunc func(*provider) error

// provider is the provider for metric.
type provider struct {
	metricExporter  sdkmetric.Exporter
	resourceOptions []resource.Option
}

// NewProvider creates a new provider with default options.
func newProvider(
	serviceName string,
	options ...ProviderOptionFunc,
) (*provider, error) {
	pvd := &provider{
		resourceOptions: []resource.Option{
			resource.WithAttributes(
				// the service name used to display metrics in backends
				semconv.ServiceNameKey.String(serviceName),
			),
		},
	}

	// push options supplied as arguments
	for _, option := range options {
		if err := option(pvd); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Defaults
	// metric exporter default
	if pvd.metricExporter == nil {
		// there should be no error possible, but just in case the lib change in the future
		me, err := stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}

		pvd.metricExporter = me
	}

	return pvd, nil
}

// Init initializes the provider.
func (pvd *provider) init(ctx context.Context) (func() error, error) {
	res, err := resource.New(ctx, pvd.resourceOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(pvd.metricExporter)),
	)
	SetMeterProvider(meterProvider)

	return func() error {
		// Shutdown will flush any remaining spans and shut down the exporter.
		if err := meterProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown MetricProvider: %w", err)
		}

		return nil
	}, nil
}
