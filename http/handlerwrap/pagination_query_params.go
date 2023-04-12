package handlerwrap

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// Query parameter keys used for cursor-based pagination.
const (
	StartingAfterKey = "starting_after"
	EndingBeforeKey  = "ending_before"
	LimitKey         = "limit"
)

// ForwardPagination and BackwardPagination indicate the direction of pagination.
const (
	ForwardPagination  = "forward"
	BackwardPagination = "backward"
)

// PaginationParams are the query parameters required for cursor-based pagination.
type PaginationParams struct {
	CursorValue     string
	CursorColumn    string
	CursorDirection string
	Limit           int
}

// NewPaginationParams creates new PaginationParams.
func NewPaginationParams(val, col, direction string, limit int) *PaginationParams {
	return &PaginationParams{
		CursorValue:     val,
		CursorColumn:    col,
		CursorDirection: direction,
		Limit:           limit,
	}
}

// PaginationParamError is the error that is returned when
// both starting_after and ending_before query parameters are provided.
type PaginationParamError struct {
	StartingAfterValue string
	EndingBeforeValue  string
}

func (e *PaginationParamError) Error() string {
	return fmt.Sprintf(
		"failed to parse query parameters starting_after: `%s` and ending_before: `%s`, should be mutually exclusive",
		e.StartingAfterValue, e.EndingBeforeValue)
}

func (e *PaginationParamError) ToErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Err:          e,
		StatusCode:   http.StatusBadRequest,
		Error:        "pagination_param_error",
		ErrorMessage: e.Error(),
	}
}

func (e *PaginationParamError) Is(err error) bool {
	var check *PaginationParamError

	if !errors.As(err, &check) {
		return false
	}

	return e.StartingAfterValue == check.StartingAfterValue &&
		e.EndingBeforeValue == check.EndingBeforeValue
}

// ParseLimitError is the error that is returned when the limit query parameter is invalid.
type ParseLimitError struct {
	Value    string
	MaxLimit int
}

func (e *ParseLimitError) Error() string {
	return fmt.Sprintf("failed to parse query param `limit`: `%s` should be a valid int between 1 and %d",
		e.Value, e.MaxLimit)
}

func (e *ParseLimitError) ToErrorResponse() *ErrorResponse {
	return &ErrorResponse{
		Err:          e,
		StatusCode:   http.StatusBadRequest,
		Error:        "parse_limit_error",
		ErrorMessage: e.Error(),
	}
}

func (e *ParseLimitError) Is(err error) bool {
	var check *ParseLimitError

	if !errors.As(err, &check) {
		return false
	}

	return e.Value == check.Value && e.MaxLimit == check.MaxLimit
}

// ParsePaginationQueryParams parses query parameters: starting_after, ending_before and
// limit from a URL and returns the corresponding PaginationParams.
//
// starting_after and ending_before are object IDs that define the place in the list and are optional.
// starting_after is used to fetch the next page of the list (forward pagination) while
// ending_before is used to fetch the previous page of the list (backward pagination).
// Returns error if both keys are used together.
// If no value is provided, PaginationParams.CursorValue will be set to the empty string.
//
// limit is the number of objects to be returned and is optional.
// Returns error if limit is not a valid integer between 1 and maxLimit.
// If no value is provided, PaginationParams.Limit will be set to defaultLimit.
func ParsePaginationQueryParams(
	urlValue *url.URL, paginationColumn string, defaultLimit, maxLimit int,
) (*PaginationParams, *ErrorResponse) {
	// defaults
	value := ""
	limit := defaultLimit
	direction := ForwardPagination

	// get values from querystring
	q := urlValue.Query()
	startingAfterValue := q.Get(StartingAfterKey)
	endingBeforeValue := q.Get(EndingBeforeKey)
	limitValue := q.Get(LimitKey)

	// check for mutually exclusive keys
	if startingAfterValue != "" && endingBeforeValue != "" {
		paginationParamErr := &PaginationParamError{
			StartingAfterValue: startingAfterValue,
			EndingBeforeValue:  endingBeforeValue,
		}

		return nil, paginationParamErr.ToErrorResponse()
	}

	if startingAfterValue != "" {
		value = startingAfterValue
		direction = ForwardPagination
	}

	if endingBeforeValue != "" {
		value = endingBeforeValue
		direction = BackwardPagination
	}

	if limitValue != "" {
		var err error

		limit, err = strconv.Atoi(limitValue)
		if err != nil || limit < 1 || limit > maxLimit {
			parseLimitErr := &ParseLimitError{Value: limitValue, MaxLimit: maxLimit}

			return nil, parseLimitErr.ToErrorResponse()
		}
	}

	return NewPaginationParams(value, paginationColumn, direction, limit), nil
}
