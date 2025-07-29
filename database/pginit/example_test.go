package pginit_test

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"

	"github.com/induzo/gocom/database/pginit/v2"
)

//nolint:testableexamples // cannot run without db
func ExamplePGInit_ConnPool() {
	pgi, err := pginit.New(
		"postgres://postgres:postgres@localhost:5432/datawarehouse?sslmode=disable&pool_max_conns=10&pool_max_conn_lifetime=1m",
	)
	if err != nil {
		log.Fatalf("init pgi config: %v", err)
	}

	ctx := context.Background()

	pool, err := pgi.ConnPool(ctx)
	if err != nil {
		log.Fatalf("init pgi config: %v", err)
	}

	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping: %v", err)
	}
}

//nolint:testableexamples // cannot run without db
func ExamplePGInit_ConnPool_withlogger() {
	textHandler := slog.NewTextHandler(io.Discard, nil)
	logger := slog.New(textHandler)

	pgi, err := pginit.New(
		"postgres://postgres:postgres@localhost:5432/datawarehouse?sslmode=disable&pool_max_conns=10&pool_max_conn_lifetime=1m",
		pginit.WithLogger(logger, "request-id"),
		pginit.WithDecimalType(),
		pginit.WithUUIDType(),
	)
	if err != nil {
		log.Fatalf("init pgi config: %v", err)
	}

	ctx := context.Background()

	pool, err := pgi.ConnPool(ctx)
	if err != nil {
		log.Fatalf("init pgi config: %v", err)
	}

	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("ping: %v", err)
	}
}

// Using standard net/http package. We can also simply pass healthCheck as a CheckFn in gocom/http/health/v2.
//
//nolint:testableexamples // cannot run without db
func ExampleConnPoolHealthCheck() {
	pgi, err := pginit.New(
		"postgres://postgres:postgres@localhost:5432/datawarehouse?sslmode=disable&pool_max_conns=10&pool_max_conn_lifetime=1m",
	)
	if err != nil {
		log.Fatalf("init pgi config: %v", err)
	}

	ctx := context.Background()

	pool, err := pgi.ConnPool(ctx)
	if err != nil {
		log.Fatalf("init pgi config: %v", err)
	}

	defer pool.Close()

	healthCheck := pginit.ConnPoolHealthCheck(pool)

	mux := http.NewServeMux()

	mux.HandleFunc("/sys/health", func(rw http.ResponseWriter, _ *http.Request) {
		if err := healthCheck(ctx); err != nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
		}
	})
}
