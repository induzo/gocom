package idempotency

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMissingIdempotencyKeyHeaderError_Error(t *testing.T) {
	t.Parallel()

	err := MissingIdempotencyKeyHeaderError{
		RequestContext: RequestContext{
			Key:       "key",
			KeyHeader: DefaultIdempotencyKeyHeader,
		},
	}

	if err.Error() != "missing idempotency key header `X-Idempotency-Key`" {
		t.Errorf("error method returned unexpected value: %s", err.Error())
	}
}

func TestRequestStillInFlightError_Error(t *testing.T) {
	t.Parallel()

	err := &RequestInFlightError{
		RequestContext: RequestContext{
			Key:       "key",
			KeyHeader: DefaultIdempotencyKeyHeader,
		},
	}

	if err.Error() != "request with key `X-Idempotency-Key`:`key` still in flight" {
		t.Errorf("error method returned unexpected value: %s", err.Error())
	}
}

func TestMismatchedSignatureError_Error(t *testing.T) {
	// TestMismatchedSignatureError_Error tests the Error method.
	t.Parallel()

	err := &MismatchedSignatureError{
		RequestContext: RequestContext{
			Key:       "key",
			KeyHeader: DefaultIdempotencyKeyHeader,
		},
	}

	if err.Error() != "mismatched signature for request with key `X-Idempotency-Key`:`key`" {
		t.Errorf("error method returned unexpected value: %s", err.Error())
	}
}

func TestStoreResponseError_Error(t *testing.T) {
	// TestStoreResponseError_Error tests the Error method.
	t.Parallel()

	err := StoreResponseError{
		RequestContext: RequestContext{
			Key:       "key",
			KeyHeader: DefaultIdempotencyKeyHeader,
		},
		Err: errors.New("test"),
	}

	if err.Error() != "error storing response: test" {
		t.Errorf("error method returned unexpected value: %s", err.Error())
	}
}

func TestStoreResponseError_Unwrap(t *testing.T) {
	// TestStoreResponseError_Unwrap tests the Unwrap method.
	t.Parallel()

	err := StoreResponseError{
		RequestContext: RequestContext{
			Key:       "key",
			KeyHeader: DefaultIdempotencyKeyHeader,
		},
		Err: errors.New("test"),
	}

	if err.Unwrap().Error() != "test" {
		t.Errorf("unwrap method returned unexpected value: %s", err.Unwrap().Error())
	}
}

func TestGetStoredResponseError_Error(t *testing.T) {
	// TestGetStoredResponseError_Error tests the Error method.
	t.Parallel()

	err := GetStoredResponseError{
		RequestContext: RequestContext{
			Key:       "key",
			KeyHeader: DefaultIdempotencyKeyHeader,
		},
		Err: errors.New("test"),
	}

	if err.Error() != "error getting stored response: test" {
		t.Errorf("error method returned unexpected value: %s", err.Error())
	}
}

func TestGetStoredResponseError_Unwrap(t *testing.T) {
	// TestGetStoredResponseError_Unwrap tests the Unwrap method.
	t.Parallel()

	err := GetStoredResponseError{
		RequestContext: RequestContext{
			Key:       "key",
			KeyHeader: DefaultIdempotencyKeyHeader,
		},
		Err: errors.New("test"),
	}

	if err.Unwrap().Error() != "test" {
		t.Errorf("unwrap method returned unexpected value: %s", err.Unwrap().Error())
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
			name: "missing idempotency key header",
			err: MissingIdempotencyKeyHeaderError{
				RequestContext: RequestContext{
					Key:       "key",
					KeyHeader: DefaultIdempotencyKeyHeader,
				},
			},
			expectedStatusCode:   http.StatusBadRequest,
			expectedBodyContains: `"title": "missing idempotency key header"`,
		},
		{
			name: "request already in flight",
			err: RequestInFlightError{
				RequestContext: RequestContext{
					Key:       "key",
					KeyHeader: DefaultIdempotencyKeyHeader,
				},
			},
			expectedStatusCode:   http.StatusConflict,
			expectedBodyContains: `"title": "request already in flight"`,
		},
		{
			name: "mismatched signature",
			err: MismatchedSignatureError{
				RequestContext: RequestContext{
					Key:       "key",
					KeyHeader: DefaultIdempotencyKeyHeader,
				},
			},
			expectedStatusCode:   http.StatusUnprocessableEntity,
			expectedBodyContains: `"title": "mismatched signature"`,
		},
		{
			name: "store response",
			err: StoreResponseError{
				RequestContext: RequestContext{
					Key:       "key",
					KeyHeader: DefaultIdempotencyKeyHeader,
				},
				Err: errors.New("test"),
			},
			expectedStatusCode:   http.StatusOK,
			expectedBodyContains: ``,
		},
		{
			name: "get stored response",
			err: &GetStoredResponseError{
				RequestContext: RequestContext{
					Key:       "key",
					KeyHeader: DefaultIdempotencyKeyHeader,
				},
				Err: errors.New("test"),
			},
			expectedStatusCode:   http.StatusInternalServerError,
			expectedBodyContains: `"title": "internal server error"`,
		},
		{
			name: "wrapped mismatched signature",
			err: fmt.Errorf("wrapped: %w", MismatchedSignatureError{
				RequestContext: RequestContext{
					Key:       "key",
					KeyHeader: DefaultIdempotencyKeyHeader,
				},
			}),
			expectedStatusCode:   http.StatusUnprocessableEntity,
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

			ErrorToHTTPJSONProblemDetail(respWriter, nil, tt.err)

			if respWriter.Code != tt.expectedStatusCode {
				t.Errorf("unexpected status code: %d", respWriter.Code)
			}

			if !strings.Contains(respWriter.Body.String(), tt.expectedBodyContains) {
				t.Errorf("unexpected body: %s", respWriter.Body.String())
			}
		})
	}
}
