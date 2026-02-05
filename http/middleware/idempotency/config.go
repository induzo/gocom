package idempotency

import (
	"net/http"
	"time"
)

const (
	DefaultIdempotencyKeyHeader             = "X-Idempotency-Key"
	DefaultIdempotentReplayedResponseHeader = "X-Idempotent-Replayed"
	DefaultResponseTTL                      = 24 * time.Hour
)

type ErrorToHTTPFn func(http.ResponseWriter, *http.Request, error)

// UserIDExtractorFn extracts the user/tenant ID from the request context.
// Return empty string if no user context is available.
type UserIDExtractorFn func(*http.Request) string

type config struct {
	idempotencyKeyIsOptional bool
	idempotencyKeyHeader     string
	idempotentReplayedHeader string
	fingerprinterFn          func(*http.Request) ([]byte, error)
	errorToHTTPFn            ErrorToHTTPFn
	affectedMethods          []string
	ignoredURLPaths          []string
	responseTTL              time.Duration
	userIDExtractor          UserIDExtractorFn
	allowedReplayHeaders     []string
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
		responseTTL:              DefaultResponseTTL,
		userIDExtractor:          defaultUserIDExtractor,
		allowedReplayHeaders:     defaultAllowedReplayHeaders(),
	}
}

// defaultUserIDExtractor tries to extract userid from context.
func defaultUserIDExtractor(req *http.Request) string {
	if v, ok := req.Context().Value("userid").(string); ok {
		return v
	}

	return ""
}

// defaultAllowedReplayHeaders returns safe headers to replay.
func defaultAllowedReplayHeaders() []string {
	return []string{
		"Content-Type",
		"Content-Language",
		"Cache-Control",
		"Expires",
		"Last-Modified",
		"ETag",
	}
}
