package handlerwrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestErrorResponse_render(t *testing.T) {
	t.Parallel()

	type args struct {
		err          error
		headers      map[string]string
		statusCode   int
		errorCode    string
		errorMessage string
		encoding     Encoding
	}

	tests := []struct {
		name            string
		args            args
		expectedStatus  int
		expectedBody    string
		expectedHeaders map[string]string
	}{
		{
			name: "happy path",
			args: args{
				err:          fmt.Errorf("test render"),
				headers:      map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff"},
				statusCode:   http.StatusBadRequest,
				errorCode:    "test_render",
				errorMessage: "test error user",
				encoding:     ApplicationJSON,
			},
			expectedStatus:  http.StatusBadRequest,
			expectedBody:    `{"error":"test_render","error_message":"test error user"}`,
			expectedHeaders: map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff", "content-type": "application/json"},
		},
		{
			name: "unsupported encoding",
			args: args{
				err:          fmt.Errorf("test render"),
				headers:      map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff"},
				statusCode:   http.StatusBadRequest,
				errorCode:    "test_render",
				errorMessage: "test error user",
				encoding:     Encoding("application/xml"),
			},
			expectedStatus:  http.StatusBadRequest,
			expectedBody:    `{"error":"test_render","error_message":"test error user"}`,
			expectedHeaders: map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff", "content-type": "application/json"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nr := httptest.NewRecorder()

			her := &ErrorResponse{
				Err:          tt.args.err,
				Headers:      tt.args.headers,
				StatusCode:   tt.args.statusCode,
				Error:        tt.args.errorCode,
				ErrorMessage: tt.args.errorMessage,
			}

			her.Render(context.Background(), nr, tt.args.encoding)

			resp := nr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)

				return
			}

			for header, headerValue := range tt.expectedHeaders {
				if resp.Header.Get(header) != headerValue {
					t.Errorf("expected response header %s: %s, got %s: %s", header, headerValue, header, resp.Header.Get(header))

					return
				}
			}

			body, _ := io.ReadAll(resp.Body)
			trimmedBody := strings.TrimSpace(string(body))
			if trimmedBody != tt.expectedBody {
				t.Errorf("expected body\n--%s--\ngot\n--%s--", tt.expectedBody, trimmedBody)

				return
			}
		})
	}
}

type structError struct {
	err  error
	name string
}

func (e *structError) Error() string {
	return fmt.Sprintf("%s: %v", e.name, e.err)
}

func TestErrorResponse_IsEqual(t *testing.T) {
	t.Parallel()

	type fields struct {
		Err            error
		HTTPStatusCode int
		Error          string
		ErrorMessage   string
	}

	testErr := errors.New("test render")
	testStructErr := &structError{
		err:  errors.New("test render"),
		name: "test",
	}
	testDiffStructErr := &structError{
		err:  errors.New("wgat"),
		name: "test",
	}

	refE := &ErrorResponse{
		Err:          testErr,
		StatusCode:   http.StatusBadRequest,
		Error:        "test_render",
		ErrorMessage: "test error user",
	}

	refStructE := &ErrorResponse{
		Err:          testStructErr,
		StatusCode:   http.StatusBadRequest,
		Error:        "test_render",
		ErrorMessage: "test error user",
	}

	type args struct {
		e1 *ErrorResponse
	}

	tests := []struct {
		name   string
		err    *ErrorResponse
		fields fields
		args   args
		want   bool
	}{
		{
			name: "equal sentinel",
			err:  refE,
			args: args{
				e1: &ErrorResponse{
					Err:          testErr,
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "test error user",
				},
			},
			want: true,
		},
		{
			name: "equal struct",
			err:  refStructE,
			args: args{
				e1: &ErrorResponse{
					Err:          testStructErr,
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "test error user",
				},
			},
			want: true,
		},
		{
			name: "diff struct",
			err:  refStructE,
			args: args{
				e1: &ErrorResponse{
					Err:          testDiffStructErr,
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "test error user",
				},
			},
			want: false,
		},
		{
			name: "diff error",
			err:  refE,
			args: args{
				e1: &ErrorResponse{
					Err:          fmt.Errorf("diff"),
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "test error user",
				},
			},
			want: false,
		},
		{
			name: "wrapped error",
			err:  refE,
			args: args{
				e1: &ErrorResponse{
					Err:          fmt.Errorf("wrapped: %w", testErr),
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "test error user",
				},
			},
			want: true,
		},
		{
			name: "diff http status code",
			err:  refE,
			args: args{
				e1: &ErrorResponse{
					Err:          testErr,
					StatusCode:   http.StatusInternalServerError,
					Error:        "test_render",
					ErrorMessage: "test error user",
				},
			},
			want: false,
		},
		{
			name: "diff error code",
			err:  refE,
			args: args{
				e1: &ErrorResponse{
					Err:          testErr,
					StatusCode:   http.StatusBadRequest,
					Error:        "diff",
					ErrorMessage: "test error user",
				},
			},
			want: false,
		},
		{
			name: "diff error msg",
			err:  refE,
			args: args{
				e1: &ErrorResponse{
					Err:          testErr,
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "diff",
				},
			},
			want: false,
		},
		{
			name: "diff L10NError",
			err:  refE,
			args: args{
				e1: &ErrorResponse{
					Err:          testErr,
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "test error user",
					L10NError: &L10NError{
						TitleKey:   "title",
						MessageKey: "messge",
					},
				},
			},
			want: false,
		},
		{
			name: "diff additional infos",
			err:  refE,
			args: args{
				e1: &ErrorResponse{
					Err:            testErr,
					StatusCode:     http.StatusBadRequest,
					Error:          "test_render",
					ErrorMessage:   "test error user",
					AdditionalInfo: struct{ hell string }{"hell"},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.IsEqual(tt.args.e1); got != tt.want {
				t.Errorf("ErrorResponse.IsEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorResponse_IsNil(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test render")

	tests := []struct {
		name    string
		errResp *ErrorResponse
		want    bool
	}{
		{
			name: "non nil err response",
			errResp: &ErrorResponse{
				Err:          testErr,
				StatusCode:   http.StatusBadRequest,
				Error:        "test_render",
				ErrorMessage: "test error user",
			},
			want: false,
		},
		{
			name:    "nil err response",
			errResp: nil,
			want:    true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.errResp.IsNil(); got != tt.want {
				t.Errorf("ErrorResponse.IsNil() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestErrorResponse_Log(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test render")

	tests := []struct {
		name       string
		errResp    *ErrorResponse
		wantSuffix string
	}{
		{
			name: "non nil err response",
			errResp: &ErrorResponse{
				Err:          testErr,
				StatusCode:   http.StatusBadRequest,
				Error:        "test_render",
				ErrorMessage: "test error user",
			},
			wantSuffix: ` level=ERROR msg="test error user" err="test render" error_code=test_render http_status_code=400`,
		},
		{
			name:       "nil err response",
			errResp:    nil,
			wantSuffix: "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var logBuf bytes.Buffer

			logger := slog.New(slog.NewTextHandler(&logBuf, nil))

			tt.errResp.Log(logger)

			if !strings.Contains(logBuf.String(), tt.wantSuffix) {
				t.Errorf("ErrorResponse.Log() = `%s`, want %s", logBuf.String(), tt.wantSuffix)
			}
		})
	}
}

func TestInternalServerError_Error(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test render")

	type fields struct {
		Err error
	}

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "happy path",
			fields: fields{Err: testErr},
			want:   fmt.Sprintf("internal error: %v", testErr.Error()),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &InternalServerError{
				Err: testErr,
			}

			if got := e.Error(); got != tt.want {
				t.Errorf("InternalServerError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInternalServerError_ToErrorResponse(t *testing.T) {
	t.Parallel()

	testErr := errors.New("test render")

	type fields struct {
		Err error
	}

	tests := []struct {
		name   string
		fields fields
		want   *ErrorResponse
	}{
		{
			name:   "happy path",
			fields: fields{Err: testErr},
			want: &ErrorResponse{
				Err:          &InternalServerError{Err: testErr},
				StatusCode:   http.StatusInternalServerError,
				Error:        "internal_error",
				ErrorMessage: "internal error",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &InternalServerError{
				Err: tt.fields.Err,
			}

			if got := e.ToErrorResponse(); !got.IsEqual(tt.want) {
				t.Errorf("InternalServerError.ToErrorResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotFoundError_Error(t *testing.T) {
	t.Parallel()

	type fields struct {
		Designation string
	}

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "happy path",
			fields: fields{Designation: "v"},
			want:   "no corresponding `v` has been found",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &NotFoundError{
				Designation: tt.fields.Designation,
			}

			if got := e.Error(); got != tt.want {
				t.Errorf("NotFoundError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotFoundError_ToErrorResponse(t *testing.T) {
	t.Parallel()

	type fields struct {
		Designation string
	}

	tests := []struct {
		name   string
		fields fields
		want   *ErrorResponse
	}{
		{
			name:   "happy path",
			fields: fields{Designation: "v"},
			want: &ErrorResponse{
				Err:          &NotFoundError{Designation: "v"},
				StatusCode:   http.StatusNotFound,
				Error:        "not_found",
				ErrorMessage: "no corresponding `v` has been found",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &NotFoundError{
				Designation: tt.fields.Designation,
			}

			if got := e.ToErrorResponse(); !got.IsEqual(tt.want) {
				t.Errorf("NotFoundError.ToErrorResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorResponse_AddHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		headers         map[string]string
		newHeaders      map[string]string
		expectedHeaders map[string]string
	}{
		{
			name:            "all new headers",
			headers:         map[string]string{"elle": "a"},
			newHeaders:      map[string]string{"il": "a"},
			expectedHeaders: map[string]string{"elle": "a", "il": "a"},
		},
		{
			name:            "overwrite headers",
			headers:         map[string]string{"elle": "a"},
			newHeaders:      map[string]string{"elle": "b"},
			expectedHeaders: map[string]string{"elle": "b"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			her := &ErrorResponse{
				Headers: tt.headers,
			}

			her.AddHeaders(tt.newHeaders)

			if len(her.Headers) != len(tt.expectedHeaders) {
				t.Errorf("wrong headers = %v, want %v", her.Headers, tt.expectedHeaders)
			}

			// check if all headers have the right value and are here
			for k, v := range tt.expectedHeaders {
				foundV, ok := her.Headers[k]
				if !ok {
					t.Errorf("header %s expected but not found", k)

					return
				}

				if foundV != v {
					t.Errorf("header %s has value %s, exected %s", k, foundV, v)
				}
			}
		})
	}
}

func TestNewUserErrorResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		err           error
		headers       map[string]string
		statusCode    int
		errCode       string
		msg           string
		titleKey      string
		msgKey        string
		expectedError *ErrorResponse
	}{
		{
			name:       "happy path",
			err:        errors.New("no"),
			headers:    nil,
			statusCode: http.StatusInsufficientStorage,
			errCode:    "insufficient_storage",
			msg:        "not enough space",
			titleKey:   "insufficient_storage",
			msgKey:     "insufficient_storage_msg",
			expectedError: &ErrorResponse{
				Err:          errors.New("no"),
				Headers:      nil,
				StatusCode:   http.StatusInsufficientStorage,
				Error:        "insufficient_storage",
				ErrorMessage: "not enough space",
				L10NError: &L10NError{
					TitleKey:   "insufficient_storage",
					MessageKey: "insufficient_storage_msg",
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			errResp := NewUserErrorResponse(tt.err, tt.headers, tt.statusCode, tt.errCode, tt.msg, tt.titleKey, tt.msgKey)

			if tt.statusCode != errResp.StatusCode {
				t.Errorf("statuscode expected: %v, actual: %v", tt.expectedError, errResp)
			}
			if *tt.expectedError.L10NError != *errResp.L10NError {
				t.Errorf("l10n error expected: %v, actual: %v", tt.expectedError.L10NError, errResp.L10NError)
			}
		})
	}
}

func TestParseBodyError_Error(t *testing.T) {
	t.Parallel()

	formDataErr := errors.New("parse form data error")

	type fields struct {
		Err error
	}

	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "happy path",
			fields: fields{Err: formDataErr},
			want:   fmt.Sprintf("parse body error: %v", formDataErr.Error()),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &ParseBodyError{
				Err: formDataErr,
			}

			if got := e.Error(); got != tt.want {
				t.Errorf("InternalServerError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseBodyError_ToErrorResponse(t *testing.T) {
	t.Parallel()

	formDataErr := errors.New("parse form data error")

	type fields struct {
		Err error
	}

	tests := []struct {
		name   string
		fields fields
		want   *ErrorResponse
	}{
		{
			name:   "happy path",
			fields: fields{Err: formDataErr},
			want: &ErrorResponse{
				Err:          formDataErr,
				Headers:      make(map[string]string),
				StatusCode:   http.StatusBadRequest,
				Error:        ErrCodeParsingBody,
				ErrorMessage: "error parsing body",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e := &ParseBodyError{
				Err: tt.fields.Err,
			}

			if got := e.ToErrorResponse(); !got.IsEqual(tt.want) {
				t.Errorf("InternalServerError.ToErrorResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorResponse_IsCodeEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err1 *ErrorResponse
		err2 *ErrorResponse
		want bool
	}{
		{
			name: "CodeEqual",
			err1: &ErrorResponse{
				Err:          errors.New("error msg 1"),
				StatusCode:   http.StatusBadRequest,
				Error:        "bad_request",
				ErrorMessage: "bad request message 1",
				L10NError: &L10NError{
					TitleKey:   "title_key",
					MessageKey: "msg_key",
				},
			},
			err2: &ErrorResponse{
				Err:          errors.New("error msg 2"),
				StatusCode:   http.StatusBadRequest,
				Error:        "bad_request",
				ErrorMessage: "bad request message 2",
				L10NError: &L10NError{
					TitleKey:   "title_key",
					MessageKey: "msg_key",
				},
			},
			want: true,
		},
		{
			name: "ErrorCodeNotEqual",
			err1: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Error:      "bad_request",
			},
			err2: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Error:      "bad_request_2",
			},
			want: false,
		},
		{
			name: "StatusCodeNotEqual",
			err1: &ErrorResponse{
				StatusCode: http.StatusInternalServerError,
				Error:      "bad_request",
			},
			err2: &ErrorResponse{
				StatusCode: http.StatusBadRequest,
				Error:      "bad_request",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err1.IsCodeEqual(tt.err2); got != tt.want {
				t.Errorf("IsCodeEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
