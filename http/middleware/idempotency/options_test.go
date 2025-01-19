package idempotency

import (
	"net/http"
	"testing"
)

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

func TestWithIdempotentReplayedHeader(t *testing.T) {
	// TestWithIdempotentReplayedHeader tests the WithIdempotentReplayedHeader option.
	t.Parallel()

	opt := WithIdempotentReplayedHeader("X-Id-Replayed")

	cfg := &config{}
	opt(cfg)

	if cfg.idempotentReplayedHeader != "X-Id-Replayed" {
		t.Error("WithIdempotentReplayedHeader did not set the header")
	}
}

func TestWithErrorToHTTPFn(t *testing.T) {
	// TestWithErrorToHTTPFn tests the WithErrorToHTTPFn option.
	t.Parallel()

	fn := func(http.ResponseWriter, *http.Request, error) {}

	opt := WithErrorToHTTPFn(fn)

	cfg := &config{}
	opt(cfg)

	if cfg.errorToHTTPFn == nil {
		t.Error("WithErrorToHTTPFn did not set the function")
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
