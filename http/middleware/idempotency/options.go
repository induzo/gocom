package idempotency

import (
	"net/http"
	"slices"
	"strings"
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

// WithErrorToHTTPFn sets a function to convert errors to HTTP status codes and content.
func WithErrorToHTTPFn(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return func(cfg *config) {
		cfg.errorToHTTPFn = fn
	}
}

// WithFingerprinter sets a function to build a request fingerprint.
//
// The custom fingerprinter is invoked instead of the default one and is
// fully responsible for any body buffering or size limiting. The
// WithMaxFingerprintBodyBytes option only affects the default fingerprinter.
func WithFingerprinter(fn func(*http.Request) ([]byte, error)) Option {
	return func(cfg *config) {
		cfg.fingerprinterFn = fn
	}
}

// WithMaxFingerprintBodyBytes bounds the maximum number of request body
// bytes the default fingerprinter reads. Requests with bodies larger than
// n trigger BodyTooLargeError, which the default error mapper renders as
// HTTP 413. The default is DefaultMaxFingerprintBodyBytes. Has no effect
// when WithFingerprinter is used.
func WithMaxFingerprintBodyBytes(n int64) Option {
	return func(cfg *config) {
		cfg.maxFingerprintBodyBytes = n
	}
}

// WithAffectedMethods sets the methods that are affected by idempotency.
// By default, POST only are affected. Method names are normalized to
// uppercase so the option is case-insensitive.
func WithAffectedMethods(methods ...string) Option {
	return func(cfg *config) {
		normalized := make([]string, 0, len(methods))
		for _, m := range methods {
			normalized = append(normalized, strings.ToUpper(m))
		}

		cfg.affectedMethods = normalized
	}
}

// WithIgnoredURLPaths sets the URL paths that are ignored by idempotency.
// Paths are matched case-insensitively (lowercased on entry) and
// deduplicated. By default, no URLs are ignored.
func WithIgnoredURLPaths(urlPaths ...string) Option {
	return func(cfg *config) {
		// remove duplicates, empty paths, normalize casing.
		urlPathsMap := make(map[string]struct{})

		for _, url := range urlPaths {
			if url == "" {
				continue
			}

			urlPathsMap[strings.ToLower(url)] = struct{}{}
		}

		// convert map keys to slice in deterministic order.
		urlPaths = make([]string, 0, len(urlPathsMap))
		for url := range urlPathsMap {
			urlPaths = append(urlPaths, url)
		}

		slices.Sort(urlPaths)

		cfg.ignoredURLPaths = urlPaths
	}
}

// WithUserIDExtractor sets a function to extract user/tenant ID from the request.
// This is used to scope idempotency keys to specific users/tenants.
func WithUserIDExtractor(fn UserIDExtractorFn) Option {
	return func(cfg *config) {
		cfg.userIDExtractor = fn
	}
}

// WithAllowedReplayHeaders sets the list of headers that are safe to replay.
// Only these headers will be copied from the stored response.
func WithAllowedReplayHeaders(headers ...string) Option {
	return func(cfg *config) {
		cfg.allowedReplayHeaders = headers
	}
}

// WithTracer sets a tracer function to add observability spans to the middleware.
// The tracer function receives the request and span name, and should return
// a function to end the span.
//
// Example with OpenTelemetry:
//
//	func(req *http.Request, spanName string) func() {
//		_, span := otel.Tracer("idempotency").Start(req.Context(), spanName)
//		return func() { span.End() }
//	}
func WithTracer(tracerFn TracerFn) Option {
	return func(cfg *config) {
		cfg.tracerFn = tracerFn
	}
}
