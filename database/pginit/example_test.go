package pginit_test

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"golang.org/x/exp/slog"

	"github.com/induzo/gocom/database/pginit"
)

//nolint:testableexamples // cannot run without db
func ExamplePGInit_ConnPool() {
	pgi, err := pginit.New(&pginit.Config{
		Host:         "localhost",
		Port:         "5432",
		User:         "postgres",
		Password:     "postgres",
		Database:     "datawarehouse",
		MaxConns:     10,
		MaxIdleConns: 10,
		MaxLifeTime:  1 * time.Minute,
	})
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
	textHandler := slog.NewTextHandler(io.Discard)
	logger := slog.New(textHandler)

	pgi, err := pginit.New(
		&pginit.Config{
			Host:         "localhost",
			Port:         "5432",
			User:         "postgres",
			Password:     "postgres",
			Database:     "datawarehouse",
			MaxConns:     10,
			MaxIdleConns: 10,
			MaxLifeTime:  1 * time.Minute,
		},
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

// Using standard net/http package. We can also simply pass healthCheck as a CheckFn in gocom/transport/http/health/v2.
//
//nolint:testableexamples // cannot run without db
func ExampleConnPoolHealthCheck() {
	pgi, err := pginit.New(&pginit.Config{
		Host:         "localhost",
		Port:         "5432",
		User:         "postgres",
		Password:     "postgres",
		Database:     "datawarehouse",
		MaxConns:     10,
		MaxIdleConns: 10,
		MaxLifeTime:  1 * time.Minute,
	})
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

	mux.HandleFunc("/sys/health", func(rw http.ResponseWriter, req *http.Request) {
		if err := healthCheck(ctx); err != nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
		}
	})
}
