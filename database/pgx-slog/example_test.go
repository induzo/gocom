package slog_test

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"

	slogadapter "github.com/induzo/gocom/database/pgx-slog"
)

//nolint:testableexamples // do not have testable output
func ExampleNewLogger() {
	textAdapter := slog.DiscardHandler
	logger := slog.New(textAdapter)

	pgxPool, _ := pgxpool.New(
		context.Background(),
		"postgres://postgres:postgres@localhost:5432/datawarehouse", // pragma: allowlist secret
	)

	pgxPool.Config().ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   slogadapter.NewLogger(logger),
		LogLevel: tracelog.LogLevelTrace,
	}
}
