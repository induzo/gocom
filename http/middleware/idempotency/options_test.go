package idempotency

import (
	"log/slog"
	"net/http"
	"testing"
)

func TestWithIdempotencyKeyHeader(t *testing.T) {
	// TestWithIdempotencyKeyHeader tests the WithIdempotencyKeyHeader option.
	t.Parallel()

	opt := WithIdempotencyKeyHeader("X-Id-Key")

	cfg := &config{}
	opt(cfg)

	if cfg.idempotencyKeyHeader != "X-Id-Key" {
		t.Error("WithIdempotencyKeyHeader did not set the header")
	}
}

func TestWithWhitelistedHeaders(t *testing.T) {
	// TestWithWhitelistedHeaders tests the WithWhitelistedHeaders option.
	t.Parallel()

	opt := WithWhitelistedHeaders("Content-Type", "Random-Header")

	cfg := &config{}
	opt(cfg)

	if len(cfg.whitelistedHeaders) != 2 {
		t.Error("WithWhitelistedHeaders did not set the headers")
	}
}

func TestWithScopeExtractor(t *testing.T) {
	// TestWithScopeExtractor tests the WithScopeExtractor option.
	t.Parallel()

	fn := func(_ *http.Request) string {
		return "scope"
	}

	opt := WithScopeExtractor(fn)

	cfg := &config{}
	opt(cfg)

	if cfg.scopeExtractorFn(&http.Request{}) != "scope" {
		t.Error("WithScopeExtractor did not set the function")
	}
}

func TestWithIdempotencyKeyIsOptional(t *testing.T) {
	// TestWithIdempotencyKeyIsOptional tests the WithIdempotencyKeyIsOptional option.
	t.Parallel()

	opt := WithIdempotencyKeyIsOptional(true)

	cfg := &config{}
	opt(cfg)

	if !cfg.IdempotencyKeyIsOptional {
		t.Error("WithIdempotencyKeyIsOptional did not set the optional flag")
	}
}

func TestWithErrorToHTTPFn(t *testing.T) {
	// TestWithErrorToHTTPFn tests the WithErrorToHTTPFn option.
	t.Parallel()

	fn := func(*slog.Logger, http.ResponseWriter, *http.Request, error) {}

	opt := WithErrorToHTTPFn(fn)

	cfg := &config{}
	opt(cfg)

	if cfg.errorToHTTPFn == nil {
		t.Error("WithErrorToHTTPFn did not set the function")
	}
}

func TestWithLogger(t *testing.T) {
	// TestWithLogger tests the WithLogger option.
	t.Parallel()

	logger := &slog.Logger{}

	opt := WithLogger(logger)

	cfg := &config{}
	opt(cfg)

	if cfg.logger == nil {
		t.Error("WithLogger did not set the logger")
	}
}
