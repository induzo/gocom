package pginit

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnPoolHealthCheck returns a health check function for pgxpool.Pool that can be used in health endpoint.
func ConnPoolHealthCheck(pool *pgxpool.Pool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return pool.Ping(ctx)
	}
}
