package idempotency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

const (
	internalServerErrorMsg = "internal server error"
	headerContentType      = "Content-Type"
)

type RequestContext struct {
	URL       string
	Method    string
	KeyHeader string
	Key       string
}

func (idrc RequestContext) String() string {
	return "`" + idrc.KeyHeader + "`:`" + idrc.Key + "`"
}

func (idrc RequestContext) toAttrs() []slog.Attr {
	return []slog.Attr{
		slog.String("url", idrc.URL),
		slog.String("method", idrc.Method),
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

type InvalidIdempotencyKeyError struct {
	RequestContext
	Err error
}

//nolint:gocritic //keep errors all the same
func (e InvalidIdempotencyKeyError) Error() string {
	return fmt.Sprintf("invalid idempotency key: %v", e.Err)
}

//nolint:gocritic //keep errors all the same
func (e InvalidIdempotencyKeyError) toAttrs() []slog.Attr {
	return append(e.RequestContext.toAttrs(), slog.Any("validation_error", e.Err))
}

//nolint:gocritic //keep errors all the same
func (e InvalidIdempotencyKeyError) Unwrap() error {
	return e.Err
}

type RequestInFlightError struct {
	RequestContext
}

func (e RequestInFlightError) Error() string {
	return "request with key " + e.String() + " still in flight"
}

// MismatchedSignatureError is returned when a request shares an existing
// key but does not match the previously stored request hash. The wire-level
// term used by the middleware is "request hash"; the public error type
// keeps the historical "signature" name for backwards compatibility.
type MismatchedSignatureError struct {
	RequestContext
}

func (e MismatchedSignatureError) Error() string {
	return "mismatched request hash for key " + e.String()
}

// BodyTooLargeError is returned when the request body exceeds the
// configured fingerprint body limit.
type BodyTooLargeError struct {
	RequestContext
	// Limit is the maximum number of body bytes the fingerprinter is
	// allowed to read.
	Limit int64
	// Err is the underlying sentinel; errors.Is(BodyTooLargeError{}, ErrBodyTooLarge)
	// returns true.
	Err error
}

//nolint:gocritic //keep errors all the same
func (e BodyTooLargeError) Error() string {
	return fmt.Sprintf(
		"request body exceeds %d bytes for key %s",
		e.Limit,
		e.RequestContext.String(),
	)
}

//nolint:gocritic //keep errors all the same
func (e BodyTooLargeError) Unwrap() error {
	return e.Err
}

//nolint:gocritic //keep errors all the same
func (e BodyTooLargeError) toAttrs() []slog.Attr {
	return append(
		e.RequestContext.toAttrs(),
		slog.Int64("body_limit_bytes", e.Limit),
	)
}

type StoreResponseError struct {
	RequestContext
	Err error
}

//nolint:gocritic //keep errors all the same
func (e StoreResponseError) Error() string {
	return fmt.Sprintf("error storing response: %v", e.Err)
}

//nolint:gocritic //keep errors all the same
func (e StoreResponseError) toAttrs() []slog.Attr {
	return append(e.RequestContext.toAttrs(), slog.Any("store_response_error", e.Err))
}

//nolint:gocritic //keep errors all the same
func (e StoreResponseError) Unwrap() error {
	return e.Err
}

type GetStoredResponseError struct {
	RequestContext
	Err error
}

//nolint:gocritic //keep errors all the same
func (e GetStoredResponseError) Error() string {
	return fmt.Sprintf("error getting stored response: %v", e.Err)
}

//nolint:gocritic //keep errors all the same
func (e GetStoredResponseError) toAttrs() []slog.Attr {
	return append(e.RequestContext.toAttrs(), slog.Any("get_stored_response_error", e.Err))
}

//nolint:gocritic //keep errors all the same
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
//
//nolint:cyclop // Error handling requires checking multiple error types.
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

	// use potential errors
	var (
		missingIdempotencyKeyHeaderError MissingIdempotencyKeyHeaderError
		invalidIdempotencyKeyError       InvalidIdempotencyKeyError
		requestInFlightError             RequestInFlightError
		mismatchedSignatureError         MismatchedSignatureError
		storeResponseError               StoreResponseError
		getStoredResponseError           GetStoredResponseError
		bodyTooLargeError                BodyTooLargeError
	)

	defer func() {
		// log an error with all the collected slog.Attrs
		slog.LogAttrs(ctx, slog.LevelError, "idempotency error", errorAttrs...)
	}()

	switch {
	case errors.As(err, &missingIdempotencyKeyHeaderError):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusBadRequest,
			Type:           "errors/missing-idempotency-key-header",
			Title:          "missing idempotency key header",
			Detail:         errorString,
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.String("issue", "missing idempotency key header"))

		errorAttrs = append(errorAttrs, missingIdempotencyKeyHeaderError.toAttrs()...)
	case errors.As(err, &invalidIdempotencyKeyError):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusBadRequest,
			Type:           "errors/invalid-idempotency-key",
			Title:          "invalid idempotency key",
			Detail:         errorString,
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.String("issue", "invalid idempotency key"))

		errorAttrs = append(errorAttrs, invalidIdempotencyKeyError.toAttrs()...)
	case errors.As(err, &requestInFlightError):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusConflict,
			Type:           "errors/request-already-in-flight",
			Title:          "request already in flight",
			Detail:         errorString,
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", requestInFlightError))
		errorAttrs = append(errorAttrs, requestInFlightError.toAttrs()...)
	case errors.As(err, &mismatchedSignatureError):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusUnprocessableEntity,
			Type:           "errors/mismatched-signature",
			Title:          "mismatched signature",
			Detail:         errorString,
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", mismatchedSignatureError))
		errorAttrs = append(errorAttrs, mismatchedSignatureError.toAttrs()...)
	case errors.As(err, &bodyTooLargeError):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusRequestEntityTooLarge,
			Type:           "errors/body-too-large",
			Title:          "request body too large",
			Detail:         errorString,
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", bodyTooLargeError))
		errorAttrs = append(errorAttrs, bodyTooLargeError.toAttrs()...)
	case errors.As(err, &getStoredResponseError):
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusInternalServerError,
			Type:           "errors/internal-server-error",
			Title:          internalServerErrorMsg,
			Detail:         "an internal server error occurred.",
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", getStoredResponseError))
		errorAttrs = append(errorAttrs, getStoredResponseError.toAttrs()...)
	case errors.As(err, &storeResponseError):
		// in case of a store response error, we want to log the error
		// but not change the content already written to the response
		errorAttrs = append(errorAttrs, slog.Any("issue", storeResponseError))
		errorAttrs = append(errorAttrs, storeResponseError.toAttrs()...)

		return
	default:
		pbDetail = ProblemDetail{
			HTTPStatusCode: http.StatusInternalServerError,
			Type:           "errors/internal-server-error",
			Title:          internalServerErrorMsg,
			Detail:         "an internal server error occurred.",
			Instance:       url,
		}

		errorAttrs = append(errorAttrs, slog.Any("issue", err))
	}

	resp, errJ := json.MarshalIndent(pbDetail, "", "  ")
	if errJ != nil {
		errorAttrs = append(errorAttrs, slog.Any("err_marshal", errJ))

		http.Error(respW, internalServerErrorMsg, http.StatusInternalServerError)

		return
	}

	respW.Header().Set(headerContentType, "application/problem+json")
	respW.WriteHeader(pbDetail.HTTPStatusCode)

	if _, errW := respW.Write(resp); errW != nil {
		errorAttrs = append(errorAttrs, slog.Any("err_write", errW))
	}
}
