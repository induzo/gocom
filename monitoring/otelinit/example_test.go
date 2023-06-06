package otelinit_test

import (
	"context"
	"fmt"
	"io"
	"log"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"

	"github.com/induzo/gocom/monitoring/otelinit"
	"github.com/induzo/gocom/monitoring/otelinit/trace"
)

// Init and start an otel trace and metric provider with a collector
func ExampleStart_collector() {
	ctx := context.Background()

	shutdowns, err := otelinit.Start(ctx, &otelinit.Config{
		AppName:       "simple-gohttp",
		Host:          "otlp.nr-data.net",
		Port:          4317,
		APIKey:        "123",
		IsSecure:      true,
		EnableMetrics: true,
	})
	if err != nil {
		log.Println("failed to start opentelemetry")

		return
	}

	for _, s := range shutdowns {
		shutdown := s
		defer func() {
			if errS := shutdown(ctx); errS != nil {
				log.Println("failed to shutdown")
			}
		}()
	}

	fmt.Println(err)

	// Output:
	// <nil>
}

// Init and start an otel trace provider with a collector only
func ExampleStart_collector_trace_only() {
	ctx := context.Background()

	shutdowns, err := otelinit.Start(ctx, &otelinit.Config{
		AppName:       "simple-gohttp",
		Host:          "otlp.nr-data.net",
		Port:          4317,
		APIKey:        "123",
		IsSecure:      true,
		EnableMetrics: false,
	})
	if err != nil {
		log.Println("failed to start opentelemetry")

		return
	}

	for _, s := range shutdowns {
		shutdown := s
		defer func() {
			if errS := shutdown(ctx); errS != nil {
				log.Println("failed to shutdown")
			}
		}()
	}

	fmt.Println(err)

	// Output:
	// <nil>
}

// Init and start an otel trace provider with a collector
func ExampleStartTrace_collector() {
	ctx := context.Background()

	shutdown, err := otelinit.StartTrace(ctx, &otelinit.Config{
		AppName:  "simple-gohttp",
		Host:     "otlp.nr-data.net",
		Port:     4317,
		APIKey:   "123",
		IsSecure: true,
	})
	if err != nil {
		log.Println("failed to start opentelemetry")

		return
	}

	defer func() {
		if errS := shutdown(ctx); errS != nil {
			log.Println("failed to shutdown trace")
		}
	}()

	fmt.Println(err)

	// Output:
	// <nil>
}

// Initialize a otel trace provider with a collector
func ExampleInitTraceProvider_collector() {
	ctx := context.Background()

	shutdown, err := otelinit.InitTraceProvider(
		ctx,
		"simple-gohttp",
		trace.WithGRPCTraceExporter(
			ctx,
			otlptracegrpc.WithEndpoint(fmt.Sprintf("%s:%d", "otlp.nr-data.net", 4317)),
			otlptracegrpc.WithHeaders(map[string]string{"api-key": "123"}),
			otlptracegrpc.WithCompressor("gzip"),
		),
	)
	if err != nil {
		log.Println("failed to initialize opentelemetry")

		return
	}

	defer func() {
		if errS := shutdown(); errS != nil {
			log.Println("failed to shutdown")
		}
	}()

	fmt.Println(err)

	// Output:
	// <nil>
}

// Initialize a otel trace provider and discard traces
// useful for dev
func ExampleInitTraceProvider_discardTraces() {
	ctx := context.Background()

	shutdown, err := otelinit.InitTraceProvider(
		ctx,
		"simple-gohttp",
		trace.WithWriterTraceExporter(io.Discard),
	)
	if err != nil {
		log.Println("failed to initialize opentelemetry")

		return
	}

	defer func() {
		if errS := shutdown(); errS != nil {
			log.Println("failed to shutdown")
		}
	}()

	fmt.Println(err)

	// Output:
	// <nil>
}
