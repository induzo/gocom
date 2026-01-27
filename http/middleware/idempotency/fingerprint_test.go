package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"maps"
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
			want: sha256Hash([]byte("POSThttp://example.comContent-Typeapplication/json")),
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
			want: sha256Hash(
				[]byte("POSThttp://example.comContent-Typeapplication/jsonuserid-123"),
			),
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

			maps.Copy(reqw.Header, tt.headers)

			got, err := buildRequestFingerprint(reqw)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !bytes.Equal(got, tt.want) {
				t.Errorf("buildRequestFingerprint() = %x, want %x", got, tt.want)
			}
		})
	}
}

func sha256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}
