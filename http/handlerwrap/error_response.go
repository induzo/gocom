package handlerwrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"

	"golang.org/x/exp/slog"
)

// ErrorResponse is a wrapper for the error response body to have a clean way of displaying errors.
type ErrorResponse struct {
	Err            error             `json:"-"`
	Headers        map[string]string `json:"-"`
	StatusCode     int               `json:"-"`
	Error          string            `json:"error"`
	ErrorMessage   string            `json:"error_message"`
	L10NError      *L10NError        `json:"l10n_error,omitempty"`
	AdditionalInfo interface{}       `json:"additional_info,omitempty"`
}

// L10NError is an error for localization
type L10NError struct {
	TitleKey   string `json:"title_key"`
	MessageKey string `json:"message_key"`
}

// NewErrorResponse creates a new ErrorResponse.
func NewErrorResponse(
	err error,
	headers map[string]string,
	statusCode int,
	errCode string,
	msg string,
) *ErrorResponse {
	return &ErrorResponse{
		Err:          err,
		Headers:      headers,
		StatusCode:   statusCode,
		Error:        errCode,
		ErrorMessage: msg,
	}
}

// NewUserErrorResponse create a new ErrorResponse with L10NError
func NewUserErrorResponse(
	err error,
	headers map[string]string,
	statusCode int,
	errCode string,
	msg string,
	titleKey string,
	msgKey string,
) *ErrorResponse {
	return &ErrorResponse{
		Err:          err,
		Headers:      headers,
		StatusCode:   statusCode,
		Error:        errCode,
		ErrorMessage: msg,
		L10NError: &L10NError{
			TitleKey:   titleKey,
			MessageKey: msgKey,
		},
	}
}

// AddHeaders add the headers to the error response
// it will overwrite a header if it already present, but will leave others in place
func (her *ErrorResponse) AddHeaders(headers map[string]string) {
	for k, v := range headers {
		her.Headers[k] = v
	}
}

func (her *ErrorResponse) Render(
	ctx context.Context,
	respW http.ResponseWriter,
	respEncoding Encoding,
) {
	Render(
		ctx,
		her.Headers,
		her.StatusCode,
		her,
		respEncoding,
		respW,
	)
}

func (her *ErrorResponse) Log(
	logger *slog.Logger,
) {
	if her == nil {
		return
	}

	logger.Error(
		her.ErrorMessage,
		slog.Any("err", her.Err),
		slog.String("error_code", her.Error),
		slog.Int("http_status_code", her.StatusCode),
	)
}

// IsNil will determine if it is empty or not
func (her *ErrorResponse) IsNil() bool {
	return her == nil || her.Err == nil
}

// IsEqual checks if an error response is equal to another.
// If using custom error structs in Err field, they should implement Is method for this to work.
func (her *ErrorResponse) IsEqual(errR1 *ErrorResponse) bool {
	if !errors.Is(errR1.Err, her.Err) {
		return false
	}

	if errR1.StatusCode != her.StatusCode {
		return false
	}

	if errR1.Error != her.Error {
		return false
	}

	if errR1.ErrorMessage != her.ErrorMessage {
		return false
	}

	if !reflect.DeepEqual(errR1.L10NError, her.L10NError) {
		return false
	}

	if !reflect.DeepEqual(errR1.AdditionalInfo, her.AdditionalInfo) {
		return false
	}

	return true
}

// IsCodeEqual compare the error code, status code and L10Error, etc.
// The fields might be used for client. Test the error message
// and error can easily lead to fragile test case. You can leverage this function in you testing to compare between the
// expectation and actual error response.
func (her *ErrorResponse) IsCodeEqual(errR1 *ErrorResponse) bool {
	if errR1.StatusCode != her.StatusCode {
		return false
	}

	if errR1.Error != her.Error {
		return false
	}

	if !reflect.DeepEqual(errR1.L10NError, her.L10NError) {
		return false
	}

	return true
}

// InternalServerError is an error that is returned when an internal server error occurs.
type InternalServerError struct {
	Err error
}

func (e *InternalServerError) Error() string {
	return fmt.Sprintf("internal error: %v", e.Err)
}

func (e *InternalServerError) ToErrorResponse() *ErrorResponse {
	return NewErrorResponse(
		e,
		make(map[string]string),
		http.StatusInternalServerError,
		"internal_error",
		"internal error",
	)
}

func (e *InternalServerError) Unwrap() error {
	return e.Err
}

func (e *InternalServerError) Is(err error) bool {
	var check *InternalServerError

	if !errors.As(err, &check) {
		return false
	}

	return errors.Is(err, check)
}

// NotFoundError is an error that is returned when a resource is not found.
type NotFoundError struct {
	Designation string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no corresponding `%s` has been found", e.Designation)
}

func (e *NotFoundError) ToErrorResponse() *ErrorResponse {
	return NewErrorResponse(e, make(map[string]string), http.StatusNotFound, "not_found", e.Error())
}

func (e *NotFoundError) Is(err error) bool {
	var check *NotFoundError

	if !errors.As(err, &check) {
		return false
	}

	return e.Designation == check.Designation
}

type ParseBodyError struct {
	Err error
}

func (e *ParseBodyError) Error() string {
	return fmt.Sprintf("parse body error: %v", e.Err)
}

func (e *ParseBodyError) ToErrorResponse() *ErrorResponse {
	return NewErrorResponse(
		e.Err,
		make(map[string]string),
		http.StatusBadRequest,
		ErrCodeParsingBody,
		"error parsing body",
	)
}

func (e *ParseBodyError) Unwrap() error {
	return e.Err
}
