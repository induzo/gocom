package metric

import (
	"encoding/json"
	"fmt"
	"io"

	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
)

// WithWriterMetricExporter allows you to push all metrics to an io.Writer
// you can use io.Discard if you don't want to be bothered by these
func WithWriterMetricExporter(w io.Writer) ProviderOptionFunc {
	return func(pvd *provider) error {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")

		metricExporter, err := stdoutmetric.New(stdoutmetric.WithEncoder(enc))
		if err != nil {
			return fmt.Errorf("creating discard exporter: %w", err)
		}

		pvd.metricExporter = metricExporter

		return nil
	}
}
