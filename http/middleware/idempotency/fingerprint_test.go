package idempotency

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_buildRequestFingerprint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		headers http.Header
		context map[any]any
		want    []byte
	}{
		{
			name: "request with no body",
			url:  "http://example.com",
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			want: []byte("POSThttp://example.comContent-Typeapplication/json"),
		},
		{
			name: "request with context userid",
			url:  "http://example.com",
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			context: map[any]any{
				"userid": "123",
			},
			want: []byte("POSThttp://example.comContent-Typeapplication/jsonuserid-123"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			//nolint:fatcontext // This is a test
			for k, v := range tt.context {
				ctx = context.WithValue(ctx, k, v)
			}

			reqw := httptest.NewRequestWithContext(ctx, http.MethodPost, tt.url, nil)

			for k, v := range tt.headers {
				reqw.Header[k] = v
			}

			got, err := buildRequestFingerprint(reqw)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !bytes.Equal(got, tt.want) {
				t.Errorf("buildRequestFingerprint() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}
