package idempotency

import (
	"net/http"
)

const (
	DefaultIdempotencyKeyHeader             = "X-Idempotency-Key"
	DefaultIdempotentReplayedResponseHeader = "X-Idempotent-Replayed"
)

type ErrorToHTTPFn func(http.ResponseWriter, *http.Request, error)

type config struct {
	idempotencyKeyIsOptional bool
	idempotencyKeyHeader     string
	idempotentReplayedHeader string
	fingerprinterFn          func(*http.Request) ([]byte, error)
	errorToHTTPFn            ErrorToHTTPFn
	affectedMethods          []string
	ignoredURLPaths          []string
}

func newDefaultConfig() *config {
	return &config{
		idempotencyKeyIsOptional: false,
		idempotencyKeyHeader:     DefaultIdempotencyKeyHeader,
		idempotentReplayedHeader: DefaultIdempotentReplayedResponseHeader,
		errorToHTTPFn:            ErrorToHTTPJSONProblemDetail,
		fingerprinterFn:          buildRequestFingerprint,
		affectedMethods:          []string{http.MethodPost},
		ignoredURLPaths:          []string{},
	}
}
