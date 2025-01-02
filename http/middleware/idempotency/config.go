package idempotency

import "net/http"

const defaultIdempotencyKeyHeader = "X-Idempotency-Key"

type config struct {
	IdempotencyKeyIsOptional bool
	idempotencyKeyHeader     string
	whitelistedHeaders       []string
	scopeExtractorFn         func(r *http.Request) string
	errorToHTTPFn            func(http.ResponseWriter, *http.Request, error)
}

func newDefaultConfig() *config {
	return &config{
		idempotencyKeyHeader: defaultIdempotencyKeyHeader,
		whitelistedHeaders:   []string{"Content-Type"},
		scopeExtractorFn:     func(r *http.Request) string { return "" },
		errorToHTTPFn:        ErrorToHTTPJSONProblemDetail,
	}
}
