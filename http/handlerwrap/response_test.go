package handlerwrap

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponse_render(t *testing.T) {
	t.Parallel()

	type args struct {
		body       any
		headers    map[string]string
		statusCode int
		encoding   Encoding
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
				body: struct {
					Test int `json:"test"`
				}{Test: 123},
				headers:    map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff"},
				statusCode: http.StatusCreated,
				encoding:   ApplicationJSON,
			},
			expectedStatus:  http.StatusCreated,
			expectedBody:    `{"test":123}`,
			expectedHeaders: map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff", "content-type": "application/json"},
		},
		{
			name: "empty response body",
			args: args{
				body:       nil,
				headers:    map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff"},
				statusCode: http.StatusNoContent,
				encoding:   ApplicationJSON,
			},
			expectedStatus:  http.StatusNoContent,
			expectedBody:    ``,
			expectedHeaders: map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff"},
		},
		{
			name: "unsupported encoding",
			args: args{
				body: struct {
					Test int `xml:"test"`
				}{Test: 123},
				headers:    map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff"},
				statusCode: http.StatusCreated,
				encoding:   Encoding("application/xml"),
			},
			expectedStatus:  http.StatusCreated,
			expectedBody:    `{"Test":123}`,
			expectedHeaders: map[string]string{"x-frame-options": "DENY", "x-content-type-options": "nosniff", "content-type": "application/json"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nr := httptest.NewRecorder()

			hr := &Response{
				Body:       tt.args.body,
				StatusCode: tt.args.statusCode,
				Headers:    tt.args.headers,
			}

			hr.Render(context.Background(), nr, tt.args.encoding)

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
