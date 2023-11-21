package pginit

import (
	"context"
	"fmt"
	"log/slog"

	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"

	slogadapter "github.com/induzo/gocom/database/pgx-slog"
)

// Option configures PGInit behaviour.
type Option func(*PGInit)

// PGInit provides capabilities for connect to postgres with pgx.pool.
type PGInit struct {
	pgxConf         *pgxpool.Config
	customDataTypes []func(*pgtype.Map)
}

// New initializes a PGInit using the provided Config and options. If
// opts is not provided it will initializes PGInit with default configuration.
func New(connString string, opts ...Option) (*PGInit, error) {
	pgxConf, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	pgi := &PGInit{
		pgxConf: pgxConf,
	}

	for _, opt := range opts {
		opt(pgi)
	}

	pgi.pgxConf.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		for _, customDataType := range pgi.customDataTypes {
			customDataType(conn.TypeMap())
		}

		return nil
	}

	return pgi, nil
}

// ConnPool initiates connection to database and return a pgxpool.Pool.
func (pgi *PGInit) ConnPool(ctx context.Context) (*pgxpool.Pool, error) {
	pool, err := pgxpool.NewWithConfig(ctx, pgi.pgxConf)
	if err != nil {
		return nil, fmt.Errorf("connect config: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// WithLogger Add logger to pgx. if the request context contains request id,
// can pass in the request id context key to reqIDKeyFromCtx and logger will
// log with the request id.
func WithLogger(logger *slog.Logger, _ string) Option {
	return func(pgi *PGInit) {
		pgi.pgxConf.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   slogadapter.NewLogger(logger),
			LogLevel: tracelog.LogLevelTrace,
		}
	}
}

// WithDecimalType set pgx decimal type to shopspring/decimal.
func WithDecimalType() Option {
	return func(p *PGInit) {
		p.customDataTypes = append(p.customDataTypes, pgxdecimal.Register)
	}
}

// WithUUIDType set pgx uuid type to gofrs/uuid.
func WithUUIDType() Option {
	return func(p *PGInit) {
		p.customDataTypes = append(p.customDataTypes, pgxuuid.Register)
	}
}
