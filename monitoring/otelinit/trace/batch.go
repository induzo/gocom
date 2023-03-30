package trace

import sdktrace "go.opentelemetry.io/otel/sdk/trace"

// WithBatchSize allows you to modify the batch size before it is sent
func WithBatchSize(size int) func(*provider) error {
	return func(pvd *provider) error {
		pvd.batchProcessorOptions = append(
			pvd.batchProcessorOptions,
			sdktrace.WithMaxExportBatchSize(size),
		)

		return nil
	}
}
