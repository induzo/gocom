package idempotency

import (
	"net/http"
)

type Option func(*config)

// WithOptionalIdempotencyKey sets the idempotency key to optional.
func WithOptionalIdempotencyKey() Option {
	return func(cfg *config) {
		cfg.idempotencyKeyIsOptional = true
	}
}

// WithIdempotencyKeyHeader sets the header to use for idempotency keys.
func WithIdempotencyKeyHeader(header string) Option {
	return func(cfg *config) {
		cfg.idempotencyKeyHeader = header
	}
}

// WithIdempotentReplayedHeader sets the header to use for idempotent replayed responses.
func WithIdempotentReplayedHeader(header string) Option {
	return func(cfg *config) {
		cfg.idempotentReplayedHeader = header
	}
}

// WithErrorToHTTP sets a function to convert errors to HTTP status codes and content.
func WithErrorToHTTPFn(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return func(cfg *config) {
		cfg.errorToHTTPFn = fn
	}
}

// WithFingerprinter sets a function to build a request fingerprint.
func WithFingerprinter(fn func(*http.Request) ([]byte, error)) Option {
	return func(cfg *config) {
		cfg.fingerprinterFn = fn
	}
}

// WithAffectedMethods sets the methods that are affected by idempotency.
// By default, POST only are affected.
func WithAffectedMethods(methods ...string) Option {
	return func(cfg *config) {
		cfg.affectedMethods = methods
	}
}

// WithIgnoredURLPaths sets the URL paths that are ignored by idempotency.
// By default, no URLs are ignored.
func WithIgnoredURLPaths(urlPaths ...string) Option {
	return func(cfg *config) {
		// remove duplicates and empty paths
		urlPathsMap := make(map[string]struct{})

		for _, url := range urlPaths {
			if url == "" {
				continue
			}

			urlPathsMap[url] = struct{}{}
		}

		// convert map keys to slice
		urlPaths = make([]string, 0, len(urlPathsMap))
		for url := range urlPathsMap {
			urlPaths = append(urlPaths, url)
		}

		cfg.ignoredURLPaths = urlPaths
	}
}
