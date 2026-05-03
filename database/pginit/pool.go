package pginit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/exaring/otelpgx"
	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	pgxdecimal "github.com/jackc/pgx-shopspring-decimal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	pgxGoogleUUID "github.com/vgarvardt/pgx-google-uuid/v5"

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
//
// If a custom-type option (WithDecimalType, WithUUIDType, WithGoogleUUIDType)
// or a connection string that ships its own AfterConnect hook is in play,
// New chains them so both run on every new physical connection.
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

	prevAfterConnect := pgi.pgxConf.AfterConnect
	if len(pgi.customDataTypes) > 0 || prevAfterConnect != nil {
		pgi.pgxConf.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
			if prevAfterConnect != nil {
				if errPrev := prevAfterConnect(ctx, conn); errPrev != nil {
					return errPrev
				}
			}

			for _, customDataType := range pgi.customDataTypes {
				customDataType(conn.TypeMap())
			}

			return nil
		}
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
//
// Note: WithLogger and WithTracer both set ConnConfig.Tracer; only the last
// one applied takes effect. To use both, wrap them externally before calling
// New.
func WithLogger(logger *slog.Logger, _ string) Option {
	return func(pgi *PGInit) {
		pgi.pgxConf.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   slogadapter.NewLogger(logger),
			LogLevel: tracelog.LogLevelTrace,
		}
	}
}

// WithTracer Add tracer to pgx.
//
// Note: WithTracer and WithLogger both set ConnConfig.Tracer; only the last
// one applied takes effect. To use both, wrap them externally before calling
// New.
func WithTracer(opts ...otelpgx.Option) Option {
	return func(pgi *PGInit) {
		pgi.pgxConf.ConnConfig.Tracer = otelpgx.NewTracer(opts...)
	}
}

// WithDecimalType set pgx decimal type to shopspring/decimal.
func WithDecimalType() Option {
	return func(pgi *PGInit) {
		pgi.customDataTypes = append(pgi.customDataTypes, pgxdecimal.Register)
	}
}

// WithUUIDType set pgx uuid type to gofrs/uuid.
func WithUUIDType() Option {
	return func(pgi *PGInit) {
		pgi.customDataTypes = append(pgi.customDataTypes, pgxuuid.Register)
	}
}

// WithGoogleUUIDType set pgx uuid type to google/uuid.
func WithGoogleUUIDType() Option {
	return func(pgi *PGInit) {
		pgi.customDataTypes = append(pgi.customDataTypes, pgxGoogleUUID.Register)
	}
}
