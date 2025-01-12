package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

type RequestContext struct {
	KeyHeader string
	Key       string
}

func (idrc RequestContext) String() string {
	return "`" + idrc.KeyHeader + "`:`" + idrc.Key + "`"
}

func (idrc RequestContext) toAttrs() []slog.Attr {
	return []slog.Attr{
		slog.String("idempotency_key_header", idrc.KeyHeader),
		slog.String("idempotency_key", idrc.Key),
	}
}

type MissingIdempotencyKeyHeaderError struct {
	RequestContext
}

func (e MissingIdempotencyKeyHeaderError) Error() string {
	return "missing idempotency key header `" + e.KeyHeader + "`"
}

type RequestInFlightError struct {
	RequestContext
}

func (e RequestInFlightError) Error() string {
	return "request with key " + e.RequestContext.String() + " still in flight"
}

type MismatchedSignatureError struct {
	RequestContext
}

func (e MismatchedSignatureError) Error() string {
	return "mismatched signature for request with key " + e.RequestContext.String()
}

type StoreResponseError struct {
	RequestContext
	Err error
}

func (e StoreResponseError) Error() string {
	return fmt.Sprintf("error storing response: %v", e.Err)
}

func (e StoreResponseError) toAttrs() []slog.Attr {
	return append(e.RequestContext.toAttrs(), slog.Any("store_response_error", e.Err))
}

func (e StoreResponseError) Unwrap() error {
	return e.Err
}

type GetStoredResponseError struct {
	RequestContext
	Err error
}

func (e GetStoredResponseError) Error() string {
	return fmt.Sprintf("error getting stored response: %v", e.Err)
}

func (e GetStoredResponseError) toAttrs() []slog.Attr {
	return append(e.RequestContext.toAttrs(), slog.Any("get_stored_response_error", e.Err))
}

func (e GetStoredResponseError) Unwrap() error {
	return e.Err
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
	respW http.ResponseWriter,
	req *http.Request,
	err error,
) {
	var pbDetail ProblemDetail

	url := ""
	ctx := context.Background() //nolint:contextcheck // context.Background() is a valid default value

	if req != nil {
		url = req.URL.String()
		ctx = req.Context()
	}

	errorString := err.Error()

	errorAttrs := []slog.Attr{}

	defer func() {
		// log an error with all the collected slog.Attrs
		slog.LogAttrs(ctx, slog.LevelError, "idempotency error", errorAttrs...)
	}()

	//nolint:errorlint // we don't use errors.As because we need to handle the specific error types and use their methods
	switch exactErr := err.(type) {
	case MissingIdempotencyKeyHeaderError:
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusBadRequest,
			Type:           "errors/missing-idempotency-key-header",
			Title:          "missing idempotency key header",
			Detail:         errorString,
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.String("issue", "missing idempotency key header"))
		errorAttrs = append(errorAttrs, exactErr.toAttrs()...)
	case RequestInFlightError:
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusConflict,
			Type:           "errors/request-already-in-flight",
			Title:          "request already in flight",
			Detail:         errorString,
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", exactErr))
		errorAttrs = append(errorAttrs, exactErr.toAttrs()...)
	case MismatchedSignatureError:
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusBadRequest,
			Type:           "errors/mismatched-signature",
			Title:          "mismatched signature",
			Detail:         errorString,
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", exactErr))
		errorAttrs = append(errorAttrs, exactErr.toAttrs()...)
	case GetStoredResponseError:
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusInternalServerError,
			Type:           "errors/internal-server-error",
			Title:          "internal server error",
			Detail:         "an internal server error occurred.",
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", exactErr))
		errorAttrs = append(errorAttrs, exactErr.toAttrs()...)
	case StoreResponseError:
		// in case of a store response error, we want to log the error
		// but not change the content already written to the response
		errorAttrs = append(errorAttrs, slog.Any("issue", exactErr))
		errorAttrs = append(errorAttrs, exactErr.toAttrs()...)

		return
	default:
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusInternalServerError,
			Type:           "errors/internal-server-error",
			Title:          "internal server error",
			Detail:         "an internal server error occurred.",
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", err))
	}

	resp, errJ := json.MarshalIndent(pbDetail, "", "  ")
	if errJ != nil {
		errorAttrs = append(errorAttrs, slog.Any("err_marshal", errJ))

		http.Error(respW, "internal server error", http.StatusInternalServerError)

		return
	}

	respW.Header().Set("Content-Type", "application/problem+json")
	respW.WriteHeader(pbDetail.HTTPStatusCode)

	if _, errW := respW.Write(resp); errW != nil {
		errorAttrs = append(errorAttrs, slog.Any("err_write", errW))
	}
}
