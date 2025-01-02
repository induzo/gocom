package idempotency

import "net/http"

// WithIdempotencyKeyHeader sets the header to use for idempotency keys.
func WithIdempotencyKeyHeader(header string) func(*config) {
	return func(c *config) {
		c.idempotencyKeyHeader = header
	}
}

// WithWhitelistedHeaders sets the headers to include in the request signature.
func WithWhitelistedHeaders(headers ...string) func(*config) {
	return func(c *config) {
		c.whitelistedHeaders = headers
	}
}

// WithScopeExtractor sets a function to extract a scope from the request.
func WithScopeExtractor(fn func(r *http.Request) string) func(*config) {
	return func(c *config) {
		c.scopeExtractorFn = fn
	}
}

// WithIdempotencyKeyIsOptional sets whether the idempotency key is optional.
func WithIdempotencyKeyIsOptional(optional bool) func(*config) {
	return func(c *config) {
		c.IdempotencyKeyIsOptional = optional
	}
}

// WithErrorToHTTP sets a function to convert errors to HTTP status codes and content.
func WithErrorToHTTPFn(fn func(http.ResponseWriter, *http.Request, error)) func(*config) {
	return func(c *config) {
		c.errorToHTTPFn = fn
	}
}
