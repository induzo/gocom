package contextslogger_test

import (
	"context"
	"io"

	slog "golang.org/x/exp/slog"

	"github.com/induzo/gocom/contextslogger"
)

//nolint:testableexamples // do not have testable output
func ExampleNewContext() {
	textHandler := slog.NewTextHandler(io.Discard)
	logger := slog.New(textHandler)

	_ = contextslogger.NewContext(context.Background(), logger)
}

//nolint:testableexamples // do not have testable output
func ExampleFromContext() {
	textHandler := slog.NewTextHandler(io.Discard)
	logger := slog.New(textHandler)

	ctxWithLogger := contextslogger.NewContext(context.Background(), logger)

	retrievedLogger := contextslogger.FromContext(ctxWithLogger)

	_ = retrievedLogger
}
