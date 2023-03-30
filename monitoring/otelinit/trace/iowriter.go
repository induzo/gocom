package trace

import (
	"fmt"
	"io"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
)

// WithWriterTraceExporter allows you to push all traces to an io.Writer
// you can use io.Discard if you don't want to be bothered by these
func WithWriterTraceExporter(w io.Writer) func(*provider) error {
	return func(pvd *provider) error {
		traceExporter, err := stdouttrace.New(stdouttrace.WithWriter(w))
		if err != nil {
			return fmt.Errorf("creating discard exporter: %w", err)
		}

		pvd.traceExporter = traceExporter

		return nil
	}
}
