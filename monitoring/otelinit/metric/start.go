package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
)

func Start(
	ctx context.Context,
	appName string,
	host string,
	port int,
	apiKey string,
	isSecure bool,
) (func(context.Context) error, error) {
	options := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(
			fmt.Sprintf("%s:%d", host, port),
		),
		otlpmetricgrpc.WithCompressor("gzip"),
	}

	if apiKey != "" {
		options = append(
			options,
			otlpmetricgrpc.WithHeaders(map[string]string{"api-key": apiKey}),
		)
	}

	if !isSecure {
		options = append(
			options,
			otlpmetricgrpc.WithInsecure(),
		)
	}

	shutdownOtel, err := InitProvider(ctx, appName, WithGRPCMetricExporter(ctx, options...))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize opentelemetry metric: %w", err)
	}

	return func(ctx context.Context) error {
		if err := shutdownOtel(); err != nil {
			return fmt.Errorf("otel metric client shutdown with err: %w", err)
		}

		return nil
	}, nil
}
