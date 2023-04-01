package pginit

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnPoolHealthCheck returns a health check function for pgxpool.Pool that can be used in health endpoint.
func ConnPoolHealthCheck(pool *pgxpool.Pool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return pool.Ping(ctx) //nolint:wrapcheck // health check fn
	}
}

// StdConnHealthCheck returns a health check function for sql.DB that can be used in health endpoint.
func StdConnHealthCheck(conn *sql.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return conn.PingContext(ctx) //nolint:wrapcheck // health check fn
	}
}
