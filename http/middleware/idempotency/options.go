package idempotency

import (
	"net/http"
)

type Option func(*config)

// WithOptionalIdempotencyKey sets the idempotency key to optional.
func WithOptionalIdempotencyKey() Option {
	return func(c *config) {
		c.idempotencyKeyIsOptional = true
	}
}

// WithIdempotencyKeyHeader sets the header to use for idempotency keys.
func WithIdempotencyKeyHeader(header string) Option {
	return func(c *config) {
		c.idempotencyKeyHeader = header
	}
}

// WithIdempotentReplayedHeader sets the header to use for idempotent replayed responses.
func WithIdempotentReplayedHeader(header string) Option {
	return func(c *config) {
		c.idempotentReplayedHeader = header
	}
}

// WithErrorToHTTP sets a function to convert errors to HTTP status codes and content.
func WithErrorToHTTPFn(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return func(c *config) {
		c.errorToHTTPFn = fn
	}
}

// WithFingerprinter sets a function to build a request fingerprint.
func WithFingerprinter(fn func(*http.Request) ([]byte, error)) Option {
	return func(c *config) {
		c.fingerprinterFn = fn
	}
}

// WithAffectedMethods sets the methods that are affected by idempotency.
// By default, POST only are affected.
func WithAffectedMethods(methods ...string) Option {
	return func(c *config) {
		c.affectedMethods = methods
	}
}
