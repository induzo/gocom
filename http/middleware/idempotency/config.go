package idempotency

import (
	"log/slog"
	"net/http"
)

const DefaultIdempotencyKeyHeader = "X-Idempotency-Key"

type config struct {
	idempotencyKeyIsOptional bool
	idempotencyKeyHeader     string
	fingerprinterFn          func(req *http.Request) ([]byte, error)
	errorToHTTPFn            func(*slog.Logger, http.ResponseWriter, *http.Request, string, error)
	logger                   *slog.Logger
}

func newDefaultConfig() *config {
	return &config{
		idempotencyKeyHeader: DefaultIdempotencyKeyHeader,
		errorToHTTPFn:        ErrorToHTTPJSONProblemDetail,
		logger:               slog.Default(),
		fingerprinterFn:      buildRequestFingerprint,
	}
}
