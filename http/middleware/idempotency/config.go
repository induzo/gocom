package idempotency

import (
	"log/slog"
	"net/http"
)

const defaultIdempotencyKeyHeader = "X-Idempotency-Key"

type config struct {
	IdempotencyKeyIsOptional bool
	idempotencyKeyHeader     string
	whitelistedHeaders       []string
	scopeExtractorFn         func(*http.Request) string
	errorToHTTPFn            func(*slog.Logger, http.ResponseWriter, *http.Request, error)
	logger                   *slog.Logger
}

func newDefaultConfig() *config {
	return &config{
		idempotencyKeyHeader: defaultIdempotencyKeyHeader,
		whitelistedHeaders:   []string{"Content-Type"},
		scopeExtractorFn:     func(*http.Request) string { return "" },
		errorToHTTPFn:        ErrorToHTTPJSONProblemDetail,
		logger:               slog.Default(),
	}
}
