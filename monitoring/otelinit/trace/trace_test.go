// This package allows you to init and enable tracing in your app
package trace

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/goleak"
)

func TestInitProvider(t *testing.T) { //nolint: paralleltest // tracer uses global vars
	tests := []struct {
		name    string
		options []ProviderOptionFunc
		wantErr bool
	}{
		{
			name:    "expecting traces if it is a correct writer",
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

	for _, tt := range tests { //nolint: paralleltest // tracer uses global vars
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			ctx := context.Background()

			options := append(tt.options, WithWriterTraceExporter(buf))

			sd, err := InitProvider(
				ctx, tt.name,
				options...,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitProvider() expected error %t got %v", tt.wantErr, err)

				return
			}

			if !tt.wantErr {
				tracer := otel.Tracer(tt.name)

				// work begins
				_, span := tracer.Start(ctx, "t")
				span.End()

				// shutdown tracer to force flush
				if sd != nil {
					if errP := sd(); errP != nil {
						t.Errorf("error shutdown: %v", errP)
					}
				}

				time.Sleep(100 * time.Millisecond)

				trs, _ := io.ReadAll(buf)
				if len(trs) == 0 {
					t.Errorf("no traces")
				}
			}
		})
	}
}

func BenchmarkInitProvider(b *testing.B) {
	for i := 0; i <= b.N; i++ {
		sd, _ := InitProvider(
			context.Background(), "bench",
			WithWriterTraceExporter(io.Discard),
		)

		b.StopTimer()

		_ = sd()

		b.StartTimer()
	}
}

func TestMain(m *testing.M) {
	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	if *leak {
		goleak.VerifyTestMain(m)

		return
	}

	os.Exit(m.Run())
}
