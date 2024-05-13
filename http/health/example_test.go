package health_test

import (
	"context"
	"net/http"
	"time"

	"github.com/induzo/gocom/http/health"
)

// Using Health HTTP handler
//
//nolint:testableexamples // cannot run without db
func ExampleHealth() {
	checks := []health.CheckConfig{
		{
			Name:    "lambda",
			Timeout: 1 * time.Second, // Optional to specify timeout
			CheckFn: func(_ context.Context) error {
				return nil
			},
		},
	}

	mux := http.NewServeMux()

	h := health.NewHealth(health.WithChecks(checks...))

	mux.Handle(health.HealthEndpoint, h.Handler())
}
