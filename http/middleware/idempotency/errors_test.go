package idempotency

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

	err := &RequestInFlightError{
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

func TestErrorToHTTPJSONProblemDetail(t *testing.T) {
	t.Parallel()

	testc := []struct {
		name                 string
		err                  error
		expectedStatusCode   int
		expectedBodyContains string
	}{
		{
			name:                 "missing idempotency key header",
			err:                  MissingIdempotencyKeyHeaderError{},
			expectedStatusCode:   http.StatusBadRequest,
			expectedBodyContains: `"title": "missing idempotency key header"`,
		},
		{
			name:                 "request already in flight",
			err:                  RequestInFlightError{},
			expectedStatusCode:   http.StatusConflict,
			expectedBodyContains: `"title": "request already in flight"`,
		},
		{
			name:                 "mismatched signature",
			err:                  MismatchedSignatureError{},
			expectedStatusCode:   http.StatusBadRequest,
			expectedBodyContains: `"title": "mismatched signature"`,
		},
		{
			name:                 "other err",
			err:                  errors.New("test"),
			expectedStatusCode:   http.StatusInternalServerError,
			expectedBodyContains: `"title": "internal server error"`,
		},
	}

	for _, tt := range testc {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			respWriter := httptest.NewRecorder()

			ErrorToHTTPJSONProblemDetail(nil, respWriter, nil, "", tt.err)

			if respWriter.Code != tt.expectedStatusCode {
				t.Errorf("unexpected status code: %d", respWriter.Code)
			}

			if !strings.Contains(respWriter.Body.String(), tt.expectedBodyContains) {
				t.Errorf("unexpected body: %s", respWriter.Body.String())
			}
		})
	}
}
