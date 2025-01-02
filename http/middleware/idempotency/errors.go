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

type RequestStillInFlightError struct {
	Method string
	URL    string
	Key    string
}

func (e RequestStillInFlightError) Error() string {
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
func ErrorToHTTPJSONProblemDetail(logger *slog.Logger, writer http.ResponseWriter, req *http.Request, err error) {
	var pbDetail ProblemDetail

	url := req.URL.String()

	switch {
	case errors.As(err, &MissingIdempotencyKeyHeaderError{}):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusBadRequest,
			Type:           "https://example.com/errors/missing-idempotency-key-header",
			Title:          "missing idempotency key header",
			Detail:         err.Error(),
			Instance:       url,
		}
	case errors.As(err, &RequestStillInFlightError{}):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusConflict,
			Type:           "https://example.com/errors/request-already-in-flight",
			Title:          "request already in flight",
			Detail:         err.Error(),
			Instance:       url,
		}
	case errors.As(err, &MismatchedSignatureError{}):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusBadRequest,
			Type:           "https://example.com/errors/mismatched-signature",
			Title:          "mismatched signature",
			Detail:         err.Error(),
			Instance:       url,
		}
	default:
		logger.Error("unhandled error",
			slog.Any("err", err),
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
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
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.Any("err", errW),
		)
	}
}
