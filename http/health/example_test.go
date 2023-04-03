package health_test

import (
	"context"
	"net/http"
	"time"

	"github.com/induzo/gocom/database/pginit"
	"github.com/induzo/gocom/http/health"
)

// Using Health HTTP handler
//
//nolint:testableexamples // cannot run without db
func ExampleHealth() {
	ctx := context.Background()

	pgi, _ := pginit.New(
		&pginit.Config{
			Host:     "localhost",
			Port:     "5432",
			User:     "postgres",
			Password: "postgres",
			Database: "datawarehouse",
		})

	conn, _ := pgi.ConnPool(ctx)
	defer conn.Close()

	checks := []health.CheckConfig{
		{
			Name:    "pgx",
			Timeout: 1 * time.Second, // Optional to specify timeout
			CheckFn: pginit.ConnPoolHealthCheck(conn),
		},
		{
			Name: "redis",
			CheckFn: func(ctx context.Context) error {
				return nil
			},
		},
	}

	mux := http.NewServeMux()

	h := health.NewHealth(health.WithChecks(checks...))

	mux.Handle(health.HealthEndpoint, h.Handler())
}
