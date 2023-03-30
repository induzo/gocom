// This package allows you to init and enable metric in your app
package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

func InitProvider(
	ctx context.Context,
	serviceName string,
	options ...ProviderOptionFunc,
) (func() error, error) {
	pvd, err := newProvider(serviceName, options...)
	if err != nil {
		return nil, fmt.Errorf("newProvider() error = %w", err)
	}

	shutdown, err := pvd.init(ctx)
	if err != nil {
		return nil, fmt.Errorf("init() error = %w", err)
	}

	return shutdown, nil
}

func Meter(name string, opts ...metric.MeterOption) metric.Meter { //nolint: ireturn // same wrap as tracer
	return GetMeterProvider().Meter(name, opts...)
}

func GetMeterProvider() metric.MeterProvider { //nolint: ireturn // same wrap as tracer
	return global.MeterProvider()
}

func SetMeterProvider(mp metric.MeterProvider) {
	global.SetMeterProvider(mp)
}
