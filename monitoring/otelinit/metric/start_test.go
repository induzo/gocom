package metric

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/goleak"
)

var testPort string //nolint:gochecknoglobals // otel-collector dockertest info

func TestMain(m *testing.M) {
	leak := flag.Bool("leak", true, "use leak detector")
	flag.Parse()

	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "otel/opentelemetry-collector",
		Tag:        "0.64.1",
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{
			Name: "no",
		}
	})
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	resource.Expire(200)
	testPort = resource.GetPort("4317/tcp")

	if err := pool.Retry(func() error {
		metricExporter, err := otlpmetricgrpc.New(
			context.Background(),
			otlpmetricgrpc.WithInsecure(),
			otlpmetricgrpc.WithEndpoint("localhost:"+testPort),
		)
		if err != nil {
			return fmt.Errorf("failed to create grpc metricExporter: %w", err)
		}

		meterProvider := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		)

		err = meterProvider.Shutdown(context.Background())

		return err
	}); err != nil {
		log.Fatalf("Could not connect to database: %s", err)
	}

	code := m.Run()

	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	if *leak {
		if code == 0 {
			if err := goleak.Find(); err != nil {
				log.Fatalf("goleak: Errors on successful test run: %v\n", err) //nolint:revive // this is reachable

				code = 1
			}
		}
	}

	os.Exit(code)
}

func TestStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		withAPIKey bool
	}{
		{
			name:       "happy path, no api key",
			withAPIKey: false,
		},
		{
			name:       "happy path, with api key",
			withAPIKey: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			apiKey := ""
			if tt.withAPIKey {
				apiKey = "123"
			}

			port, _ := strconv.Atoi(testPort)
			sd, err := Start(ctx, tt.name, "127.0.0.1", port, apiKey, false)
			if err != nil {
				t.Errorf("error starting otel: %v", err)

				return
			}

			if sd != nil {
				if err := sd(ctx); err != nil {
					t.Errorf("error shutdown otel: %v", err)

					return
				}
			}
		})
	}
}
