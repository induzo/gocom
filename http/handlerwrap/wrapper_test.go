package handlerwrap

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	if *leak {
		goleak.VerifyTestMain(m)

		return
	}

	os.Exit(m.Run())
}

func TestWrapper(t *testing.T) {
	t.Parallel()

	type args struct {
		accept            string
		httpResponse      *Response
		httpErrorResponse *ErrorResponse
	}

	tests := []struct {
		name           string
		args           args
		expectedStatus int
		expectedBody   string
		expectedHeader map[string][]string
	}{
		{
			name: "normal path",
			args: args{
				accept: "application/json",
				httpResponse: &Response{
					Body: struct {
						Test int `json:"test"`
					}{Test: 123},
					StatusCode: http.StatusCreated,
					Headers: map[string]string{
						"Test": "test",
					},
				},
				httpErrorResponse: nil,
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"test":123}`,
			expectedHeader: map[string][]string{
				"Test":         {"test"},
				"Content-Type": {"application/json"},
			},
		},
		{
			name: "empty response body",
			args: args{
				accept: "application/json",
				httpResponse: &Response{
					StatusCode: http.StatusOK,
				},
				httpErrorResponse: nil,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "",
			expectedHeader: map[string][]string{},
		},
		{
			name: "error path",
			args: args{
				accept:       "application/json",
				httpResponse: nil,
				httpErrorResponse: &ErrorResponse{
					Err:          fmt.Errorf("test render"),
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "test error user",
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"test_render","error_message":"test error user"}`,
			expectedHeader: map[string][]string{
				"Content-Type": {"application/json"},
			},
		},
		{
			name: "error with additional infos",
			args: args{
				accept:       "application/json",
				httpResponse: nil,
				httpErrorResponse: &ErrorResponse{
					Err:          fmt.Errorf("test render"),
					StatusCode:   http.StatusBadRequest,
					Error:        "test_render",
					ErrorMessage: "test error user",
					AdditionalInfo: struct {
						Hello string `json:"hello"`
					}{"hi"},
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   `{"error":"test_render","error_message":"test error user","additional_info":{"hello":"hi"}}`,
			expectedHeader: map[string][]string{
				"Content-Type": {"application/json"},
			},
		},
		{
			name: "error with unsupported body type",
			args: args{
				accept: "application/json",
				httpResponse: &Response{
					Body:       make(chan int),
					StatusCode: http.StatusCreated,
					Headers:    map[string]string{},
				},
				httpErrorResponse: nil,
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "",
			expectedHeader: map[string][]string{},
		},
		{
			name: "no accept header - default to json",
			args: args{
				httpResponse: &Response{
					Body: struct {
						Test int `json:"test"`
					}{Test: 123},
					StatusCode: http.StatusCreated,
					Headers: map[string]string{
						"Test": "test",
					},
				},
				httpErrorResponse: nil,
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"test":123}`,
			expectedHeader: map[string][]string{
				"Test":         {"test"},
				"Content-Type": {"application/json"},
			},
		},
		{
			name: "unsupported accept header - default to json",
			args: args{
				accept: "application/xml",
				httpResponse: &Response{
					Body: struct {
						Test int `xml:"test"`
					}{Test: 123},
					StatusCode: http.StatusCreated,
					Headers: map[string]string{
						"Test": "test",
					},
				},
				httpErrorResponse: nil,
			},
			expectedStatus: http.StatusCreated,
			expectedBody:   `{"Test":123}`,
			expectedHeader: map[string][]string{
				"Test":         {"test"},
				"Content-Type": {"application/json"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, "/", &bytes.Reader{})

			if tt.args.accept != "" {
				req.Header.Add("Accept", tt.args.accept)
			}

			nr := httptest.NewRecorder()

			f := func(r *http.Request) (*Response, *ErrorResponse) {
				if tt.args.httpResponse != nil {
					return tt.args.httpResponse, nil
				}

				return nil, tt.args.httpErrorResponse
			}

			handler := Wrapper(f)

			handler.ServeHTTP(nr, req)

			resp := nr.Result()
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			body, _ := io.ReadAll(resp.Body)
			trimmedBody := strings.TrimSpace(string(body))

			if trimmedBody != tt.expectedBody {
				t.Errorf("expected body\n--%s--\ngot\n--%s--", tt.expectedBody, trimmedBody)
			}

			if !reflect.DeepEqual(resp.Header, http.Header(tt.expectedHeader)) {
				t.Errorf("expected header %s, got %s", http.Header(tt.expectedHeader), resp.Header)
			}
		})
	}
}

func BenchmarkHTTPWrapper(b *testing.B) {
	req := httptest.NewRequest(http.MethodGet, "/", &bytes.Reader{})
	nr := httptest.NewRecorder()

	f := func(r *http.Request) (*Response, *ErrorResponse) {
		return &Response{
			Body: struct {
				Test int
			}{
				Test: 123,
			},
			StatusCode: 200,
		}, nil
	}

	handler := Wrapper(f)

	b.ResetTimer()

	for i := 0; i <= b.N; i++ {
		handler.ServeHTTP(nr, req)
	}
}
