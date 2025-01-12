package idempotency

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

type MissingIdempotencyKeyHeaderError struct {
	Method         string
	URL            string
	ExpectedHeader string
}

func (e MissingIdempotencyKeyHeaderError) Error() string {
	return fmt.Sprintf("missing idempotency key header `%s` for request to %s %s", e.ExpectedHeader, e.Method, e.URL)
}

type RequestInFlightError struct {
	Method string
	URL    string
	Key    string
}

func (e RequestInFlightError) Error() string {
	return fmt.Sprintf("request to `%s %s` `%s` still in flight", e.Method, e.URL, e.Key)
}

type MismatchedSignatureError struct {
	Method string
	URL    string
	Key    string
}

func (e MismatchedSignatureError) Error() string {
	return fmt.Sprintf("mismatched signature for request to `%s %s` `%s`", e.Method, e.URL, e.Key)
}

// Conforming to RFC9457 (https://www.rfc-editor.org/rfc/rfc9457.html)
type ProblemDetail struct {
	HTTPStatusCode int `json:"-"`

	Type             string         `json:"type"`
	Title            string         `json:"title"`
	Detail           string         `json:"detail"`
	Instance         string         `json:"instance"`
	ExtensionMembers map[string]any `json:",omitempty"`
}

// ErrorToHTTPJSONProblemDetail converts an error to a RFC9457 problem detail.
// This is a sample errorToHTTPFn function that handles the specific errors encountered
// You can add your own func and set it inside the config
func ErrorToHTTPJSONProblemDetail(
	logger *slog.Logger,
	writer http.ResponseWriter,
	req *http.Request,
	key string,
	err error,
) {
	var pbDetail ProblemDetail

	method := http.MethodGet
	url := ""

	if req != nil {
		method = req.Method
		url = req.URL.String()
	}

	errorString := err.Error()

	if logger == nil {
		logger = slog.Default()
	}

	switch {
	case errors.As(err, &MissingIdempotencyKeyHeaderError{}):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusBadRequest,
			Type:           "https://example.com/errors/missing-idempotency-key-header",
			Title:          "missing idempotency key header",
			Detail:         errorString,
			Instance:       url,
		}
	case errors.As(err, &RequestInFlightError{}):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusConflict,
			Type:           "https://example.com/errors/request-already-in-flight",
			Title:          "request already in flight",
			Detail:         errorString,
			Instance:       url,
		}
	case errors.As(err, &MismatchedSignatureError{}):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusBadRequest,
			Type:           "https://example.com/errors/mismatched-signature",
			Title:          "mismatched signature",
			Detail:         errorString,
			Instance:       url,
		}
	default:
		logger.Error("unhandled error",
			slog.Any("err", err),
			slog.String("method", method),
			slog.String("url", url),
			slog.String("key", key),
		)

		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusInternalServerError,
			Type:           "https://example.com/errors/internal-server-error",
			Title:          "internal server error",
			Detail:         "an internal server error occurred.",
			Instance:       url,
		}
	}

	resp, errJ := json.MarshalIndent(pbDetail, "", "  ")
	if errJ != nil {
		http.Error(writer, "internal server error", http.StatusInternalServerError)

		return
	}

	writer.Header().Set("Content-Type", "application/problem+json")
	writer.WriteHeader(pbDetail.HTTPStatusCode)

	if _, errW := writer.Write(resp); errW != nil {
		logger.Error("failed writing response",
			slog.String("method", method),
			slog.String("url", url),
			slog.String("key", key),
			slog.Any("err", errW),
		)
	}
}
