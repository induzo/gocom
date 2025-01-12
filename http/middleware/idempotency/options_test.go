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

func TestWithOptionalIdempotencyKey(t *testing.T) {
	// TestWithOptionalIdempotencyKey tests the WithOptionalIdempotencyKey option.
	t.Parallel()

	opt := WithOptionalIdempotencyKey()

	cfg := &config{}
	opt(cfg)

	if !cfg.idempotencyKeyIsOptional {
		t.Error("WithOptionalIdempotencyKey did not set the optional flag")
	}
}

func TestWithErrorToHTTPFn(t *testing.T) {
	// TestWithErrorToHTTPFn tests the WithErrorToHTTPFn option.
	t.Parallel()

	fn := func(*slog.Logger, http.ResponseWriter, *http.Request, string, error) {}

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

func TestWithFingerprinter(t *testing.T) {
	// TestWithFingerprinter tests the WithFingerprinter option.
	t.Parallel()

	fn := func(*http.Request) ([]byte, error) { return nil, nil }

	opt := WithFingerprinter(fn)

	cfg := &config{}
	opt(cfg)

	if cfg.fingerprinterFn == nil {
		t.Error("WithFingerprinter did not set the function")
	}
}
