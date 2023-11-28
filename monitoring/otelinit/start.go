package otelinit

import (
	"context"
	"fmt"

	"github.com/induzo/gocom/monitoring/otelinit/v2/trace"
)

func Start(ctx context.Context, conf *Config) ([]func(ctx context.Context) error, error) {
	shutdownOtels := []func(ctx context.Context) error{}

	traceShutdownOtel, err := StartTrace(ctx, conf)
	if err != nil {
		return nil, err
	}

	shutdownOtels = append(shutdownOtels, traceShutdownOtel)

	return shutdownOtels, nil
}

func StartTrace(ctx context.Context, conf *Config) (func(ctx context.Context) error, error) {
	traceShutdownOtel, err := trace.Start(ctx, conf.AppName, conf.Host, conf.Port, conf.APIKey, conf.IsSecure)
	if err != nil {
		return nil, fmt.Errorf("failed to start trace: %w", err)
	}

	return traceShutdownOtel, nil
}
