package handlerwrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// NamedURLParamsGetter is the interface that is used to parse the URL parameters.
type NamedURLParamsGetter func(ctx context.Context, key string) (string, *ErrorResponse)

// MissingParamError is the error that is returned when a named URL param is missing.
type MissingParamError struct {
	Name string
}

func (e *MissingParamError) Error() string {
	return fmt.Sprintf("named URL param `%s` is missing", e.Name)
}

func (e *MissingParamError) ToErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Err:          e,
		StatusCode:   http.StatusBadRequest,
		Error:        "missing_param_error",
		ErrorMessage: e.Error(),
	}
}

func (e *MissingParamError) Is(err error) bool {
	var check *MissingParamError

	if !errors.As(err, &check) {
		return false
	}

	return e.Name == check.Name
}

// ParsingParamError is the error that is returned when a named URL param is invalid.
type ParsingParamError struct {
	Name  string
	Value string
}

func (e *ParsingParamError) Error() string {
	return fmt.Sprintf("can not parse named URL param `%s`: `%s` is invalid", e.Name, e.Value)
}

func (e *ParsingParamError) ToErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Err:          e,
		StatusCode:   http.StatusBadRequest,
		Error:        "parsing_param_error",
		ErrorMessage: e.Error(),
	}
}

func (e *ParsingParamError) Is(err error) bool {
	var check *ParsingParamError

	if !errors.As(err, &check) {
		return false
	}

	return e.Name == check.Name && e.Value == check.Value
}
