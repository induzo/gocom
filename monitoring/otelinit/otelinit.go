// This package allows you to init and enable tracing in your app
package otelinit

import (
	"context"
	"fmt"

	"github.com/induzo/gocom/monitoring/otelinit/metric"
	"github.com/induzo/gocom/monitoring/otelinit/trace"
)

func InitTraceProvider(
	ctx context.Context,
	serviceName string,
	options ...trace.ProviderOptionFunc,
) (func() error, error) {
	pvd, err := trace.InitProvider(ctx, serviceName, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to init trace provider: %w", err)
	}

	return pvd, nil
}

func InitMetricProvider(
	ctx context.Context,
	serviceName string,
	options ...metric.ProviderOptionFunc,
) (func() error, error) {
	pvd, err := metric.InitProvider(ctx, serviceName, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to init metric provider: %w", err)
	}

	return pvd, nil
}
