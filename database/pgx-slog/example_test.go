package slog_test

import (
	"context"
	"io"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"golang.org/x/exp/slog"

	slogadapter "github.com/induzo/gocom/database/pgx-slog"
)

//nolint:testableexamples // do not have testable output
func ExampleNewLogger() {
	textAdapter := slog.NewTextHandler(io.Discard, nil)
	logger := slog.New(textAdapter)

	pgxPool, _ := pgxpool.New(context.Background(), "postgres://postgres:postgres@localhost:5432/datawarehouse")

	pgxPool.Config().ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   slogadapter.NewLogger(logger),
		LogLevel: tracelog.LogLevelTrace,
	}
}
