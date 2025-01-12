package idempotency

import (
	"log/slog"
	"net/http"
)

// WithIdempotencyKeyHeader sets the header to use for idempotency keys.
func WithIdempotencyKeyHeader(header string) func(*config) {
	return func(c *config) {
		c.idempotencyKeyHeader = header
	}
}

// WithOptionalIdempotencyKey sets the idempotency key to optional.
func WithOptionalIdempotencyKey() func(*config) {
	return func(c *config) {
		c.idempotencyKeyIsOptional = true
	}
}

// WithErrorToHTTP sets a function to convert errors to HTTP status codes and content.
func WithErrorToHTTPFn(fn func(*slog.Logger, http.ResponseWriter, *http.Request, string, error)) func(*config) {
	return func(c *config) {
		c.errorToHTTPFn = fn
	}
}

// WithLogger sets the logger to use for logging.
func WithLogger(logger *slog.Logger) func(*config) {
	return func(c *config) {
		c.logger = logger
	}
}

// WithFingerprinter sets a function to build a request fingerprint.
func WithFingerprinter(fn func(*http.Request) ([]byte, error)) func(*config) {
	return func(c *config) {
		c.fingerprinterFn = fn
	}
}
