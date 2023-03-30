package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
)

func Start(
	ctx context.Context,
	appName string,
	host string,
	port int,
	apiKey string,
	isSecure bool,
) (func(ctx context.Context) error, error) {
	var err error

	options := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(
			fmt.Sprintf("%s:%d", host, port),
		),
		otlptracegrpc.WithCompressor("gzip"),
	}

	if apiKey != "" {
		options = append(
			options,
			otlptracegrpc.WithHeaders(map[string]string{"api-key": apiKey}),
		)
	}

	if !isSecure {
		options = append(
			options,
			otlptracegrpc.WithInsecure(),
		)
	}

	shutdownOtel, err := InitProvider(ctx, appName, WithGRPCTraceExporter(ctx, options...))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize opentelemetry: %w", err)
	}

	return func(ctx context.Context) error {
		if err := shutdownOtel(); err != nil {
			return fmt.Errorf("otel client shutdown with err: %w", err)
		}

		return nil
	}, nil
}
