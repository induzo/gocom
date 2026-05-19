package idempotency

import (
	"net/http"
)

const (
	DefaultIdempotencyKeyHeader             = "X-Idempotency-Key"
	DefaultIdempotentReplayedResponseHeader = "X-Idempotent-Replayed"

	// DefaultMaxFingerprintBodyBytes bounds the request body bytes the
	// default fingerprinter will read. Bodies above this size cause the
	// middleware to reject the request with BodyTooLargeError.
	DefaultMaxFingerprintBodyBytes int64 = 5 * 1024 * 1024 // 5 MiB

	// userIDCtxKeyLegacy is the historical untyped string key the default
	// extractor and fingerprinter consulted; it is still honored as a
	// fallback for backwards compatibility, behind UserIDCtxKey.
	userIDCtxKeyLegacy = "userid"
)

type userIDCtxKeyType struct{}

// UserIDCtxKey is the typed context key the default user-ID extractor and
// the default fingerprinter look up to scope idempotency to a tenant.
// Callers that want explicit control should set their own extractor via
// WithUserIDExtractor instead.
var UserIDCtxKey userIDCtxKeyType //nolint:gochecknoglobals // sentinel ctx key

type ErrorToHTTPFn func(http.ResponseWriter, *http.Request, error)

// UserIDExtractorFn extracts the user/tenant ID from the request context.
// Return empty string if no user context is available.
type UserIDExtractorFn func(*http.Request) string

// TracerFn is a function that starts a span with the given name and returns
// a function to end the span.
// This allows integration with any tracing library (OpenTelemetry, DataDog, Jaeger, etc.).
// The returned function should be called to ensure the span is ended.
type TracerFn func(req *http.Request, spanName string) func()

// ShouldStoreResponseFn decides whether a completed response should be
// persisted in the idempotency store. Return false to leave the key
// reusable (e.g. skip storage for client-error responses).
type ShouldStoreResponseFn func(statusCode int) bool

// defaultShouldStoreResponse always stores — preserves the original behaviour.
func defaultShouldStoreResponse(_ int) bool { return true }

type config struct {
	idempotencyKeyIsOptional bool
	idempotencyKeyHeader     string
	idempotentReplayedHeader string
	fingerprinterFn          func(*http.Request) ([]byte, error)
	errorToHTTPFn            ErrorToHTTPFn
	affectedMethods          []string
	ignoredURLPaths          []string
	userIDExtractor          UserIDExtractorFn
	allowedReplayHeaders     []string
	tracerFn                 TracerFn
	maxFingerprintBodyBytes  int64
	shouldStoreResponseFn    ShouldStoreResponseFn
}

func newDefaultConfig() *config {
	cfg := &config{
		idempotencyKeyIsOptional: false,
		idempotencyKeyHeader:     DefaultIdempotencyKeyHeader,
		idempotentReplayedHeader: DefaultIdempotentReplayedResponseHeader,
		errorToHTTPFn:            ErrorToHTTPJSONProblemDetail,
		affectedMethods:          []string{http.MethodPost},
		ignoredURLPaths:          []string{},
		userIDExtractor:          defaultUserIDExtractor,
		allowedReplayHeaders:     defaultAllowedReplayHeaders(),
		tracerFn:                 noOpTracer,
		maxFingerprintBodyBytes:  DefaultMaxFingerprintBodyBytes,
		shouldStoreResponseFn:    defaultShouldStoreResponse,
	}

	// Bind the default fingerprinter via a closure so that
	// WithMaxFingerprintBodyBytes (applied after newDefaultConfig) is
	// honored by the default fingerprinter at request time.
	cfg.fingerprinterFn = func(req *http.Request) ([]byte, error) {
		return buildRequestFingerprint(req, cfg.maxFingerprintBodyBytes)
	}

	return cfg
}

// noOpTracer is a no-op tracer that does nothing.
func noOpTracer(_ *http.Request, _ string) func() {
	return func() {}
}

// defaultUserIDExtractor reads the user ID from the request context. It
// prefers the typed UserIDCtxKey but falls back to the legacy "userid"
// string key for backwards compatibility.
func defaultUserIDExtractor(req *http.Request) string {
	if v, ok := req.Context().Value(UserIDCtxKey).(string); ok {
		return v
	}

	if v, ok := req.Context().Value(userIDCtxKeyLegacy).(string); ok {
		return v
	}

	return ""
}

// defaultAllowedReplayHeaders returns safe headers to replay.
func defaultAllowedReplayHeaders() []string {
	return []string{
		headerContentType,
		"Content-Language",
		"Cache-Control",
		"Expires",
		"Last-Modified",
		"ETag",
	}
}
