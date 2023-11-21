package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

// ProviderOptionFunc is the option function for provider
type ProviderOptionFunc func(*provider) error

// provider is the provider for tracing.
type provider struct {
	traceExporter         sdktrace.SpanExporter
	resourceOptions       []resource.Option
	batchProcessorOptions []sdktrace.BatchSpanProcessorOption
}

// NewProvider creates a new provider with default options.
func newProvider(
	serviceName string,
	options ...ProviderOptionFunc,
) (*provider, error) {
	pvd := &provider{
		resourceOptions: []resource.Option{
			resource.WithAttributes(
				// the service name used to display traces in backends
				semconv.ServiceNameKey.String(serviceName),
			),
		},
		batchProcessorOptions: []sdktrace.BatchSpanProcessorOption{},
	}

	// push options supplied as arguments
	for _, option := range options {
		if err := option(pvd); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Defaults
	// trace exporter default
	if pvd.traceExporter == nil {
		// there should be no error possible, but just in case the lib change in the future
		te, err := stdouttrace.New()
		if err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}

		pvd.traceExporter = te
	}

	return pvd, nil
}

// Init initializes the provider.
func (pvd *provider) init(ctx context.Context) (func() error, error) {
	res, err := resource.New(ctx, pvd.resourceOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(pvd.traceExporter, pvd.batchProcessorOptions...)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tracerProvider)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return func() error {
		// Shutdown will flush any remaining spans and shut down the exporter.
		if err := tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown TracerProvider: %w", err)
		}

		return nil
	}, nil
}
