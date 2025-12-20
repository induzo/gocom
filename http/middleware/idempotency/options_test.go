package idempotency

import (
	"net/http"
	"slices"
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

func TestWithAffectedMethods(t *testing.T) {
	// TestWithAffectedMethods tests the WithAffectedMethods option.
	t.Parallel()

	opt := WithAffectedMethods("GET", "POST")

	cfg := &config{}
	opt(cfg)

	if len(cfg.affectedMethods) != 2 {
		t.Error("WithAffectedMethods did not set the methods")
	}
}

func TestWithIgnoredURLPaths(t *testing.T) {
	// TestWithIgnoredURLs tests the WithIgnoredURLs option.
	t.Parallel()

	tests := []struct {
		name              string
		urls              []string
		expectedURLParths []string
	}{
		{
			name:              "single URL",
			urls:              []string{"/ignored"},
			expectedURLParths: []string{"/ignored"},
		},
		{
			name:              "multiple URLs",
			urls:              []string{"/ignored", "/also-ignored"},
			expectedURLParths: []string{"/ignored", "/also-ignored"},
		},
		{
			name:              "duplicate URLs",
			urls:              []string{"/ignored", "/ignored"},
			expectedURLParths: []string{"/ignored"},
		},
		{
			name:              "empty URL",
			urls:              []string{"/ignored", ""},
			expectedURLParths: []string{"/ignored"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			opt := WithIgnoredURLPaths(test.urls...)
			cfg := &config{}

			opt(cfg)

			if len(cfg.ignoredURLPaths) != len(test.expectedURLParths) {
				t.Errorf(
					"WithIgnoredURLPaths did not set the URLs correctly, got %v, want %v",
					cfg.ignoredURLPaths,
					test.expectedURLParths,
				)
			}

			for _, expectedURL := range test.expectedURLParths {
				found := slices.Contains(cfg.ignoredURLPaths, expectedURL)

				if !found {
					t.Errorf("WithIgnoredURLPaths did not set the URL %s", expectedURL)
				}
			}
		})
	}
}
