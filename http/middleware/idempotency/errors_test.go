package idempotency

import (
	"net/http"
	"testing"
)

func TestMissingIdempotencyKeyHeaderError_Error(t *testing.T) {
	// TestMissingIdempotencyKeyHeaderError_Error tests the Error method.
	t.Parallel()

	err := &MissingIdempotencyKeyHeaderError{
		Method:         http.MethodPost,
		URL:            "http://example.com",
		ExpectedHeader: "X-Id-Key",
	}

	if err.Error() != "missing idempotency key header `X-Id-Key` for request to POST http://example.com" {
		t.Errorf("error method returned unexpected value: %s", err.Error())
	}
}

func TestRequestStillInFlightError_Error(t *testing.T) {
	// TestRequestStillInFlightError_Error tests the Error method.
	t.Parallel()

	err := &RequestStillInFlightError{
		Method: http.MethodPost,
		URL:    "http://example.com",
		Key:    "key",
	}

	if err.Error() != "request to `POST http://example.com` `key` still in flight" {
		t.Errorf("error method returned unexpected value: %s", err.Error())
	}
}

func TestMismatchedSignatureError_Error(t *testing.T) {
	// TestMismatchedSignatureError_Error tests the Error method.
	t.Parallel()

	err := &MismatchedSignatureError{
		Method: http.MethodPost,
		URL:    "http://example.com",
		Key:    "key",
	}

	if err.Error() != "mismatched signature for request to `POST http://example.com` `key`" {
		t.Errorf("error method returned unexpected value: %s", err.Error())
	}
}
