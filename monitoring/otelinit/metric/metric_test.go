// This package allows you to init and enable tracing in your app
package metric

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestInitProvider(t *testing.T) { //nolint: paralleltest // meter uses global vars
	tests := []struct {
		name    string
		options []ProviderOptionFunc
		wantErr bool
	}{
		{
			name:    "expecting metrics if it is a correct writer",
			options: nil,
			wantErr: false,
		},
		{
			name: "expecting error at provider new",
			options: []ProviderOptionFunc{
				func(pvd *provider) error { return errors.New("error") },
			},
			wantErr: true,
		},
		{
			name: "expecting error at provider init",
			options: []ProviderOptionFunc{
				func(pvd *provider) error {
					pvd.resourceOptions = append(
						pvd.resourceOptions,
						resource.WithDetectors(&testBadDetector{schemaURL: "https://opentelemetry.io/schemas/1.4.0"}),
					)
					pvd.resourceOptions = append(
						pvd.resourceOptions,
						resource.WithDetectors(&testBadDetector{schemaURL: "https://opentelemetry.io/schemas/1.3.0"}),
					)

					return nil
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests { //nolint: paralleltest // meter uses global vars
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			ctx := context.Background()

			options := append(tt.options, WithWriterMetricExporter(buf))

			sd, err := InitProvider(
				ctx, tt.name,
				options...,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitProvider() expected error %t got %v", tt.wantErr, err)

				return
			}

			if !tt.wantErr {
				meter := Meter(tt.name)

				counter, _ := meter.Int64Counter(
					"test.my_counter",
					instrument.WithUnit("1"),
					instrument.WithDescription("Just a test counter"),
				)
				counter.Add(ctx, 1, attribute.String("foo", "bar"))

				// shutdown meter to force flush
				if sd != nil {
					if errP := sd(); errP != nil {
						t.Errorf("error shutdown: %v", errP)
					}
				}

				time.Sleep(100 * time.Millisecond)

				trs, _ := io.ReadAll(buf)
				if len(trs) == 0 {
					t.Errorf("no metric")
				}
			}
		})
	}
}

func BenchmarkInitProvider(b *testing.B) {
	for i := 0; i <= b.N; i++ {
		sd, _ := InitProvider(
			context.Background(), "bench",
			WithWriterMetricExporter(io.Discard),
		)

		b.StopTimer()

		_ = sd()

		b.StartTimer()
	}
}
