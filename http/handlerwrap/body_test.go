package handlerwrap

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBindBody(t *testing.T) {
	t.Parallel()

	type testTarget struct {
		Test int `json:"test"`
	}

	tests := []struct {
		name               string
		requestContentType string
		requestBody        string
		expectedTarget     any
		wantErr            bool
	}{
		{
			name:               "happy path",
			requestContentType: "application/json",
			requestBody:        `{"test":123}`,
			expectedTarget:     testTarget{Test: 123},
			wantErr:            false,
		},
		{
			name:               "wrong format",
			requestContentType: "application/json",
			requestBody:        `{"test":123`,
			expectedTarget:     testTarget{},
			wantErr:            true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(tt.requestBody)))

			req.Header.Add("Content-Type", tt.requestContentType)

			var target testTarget

			if err := BindBody(req, &target); (err != nil) != tt.wantErr {
				t.Errorf("HTTPBindBody() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && target != tt.expectedTarget {
				t.Errorf("HTTPBindBody() expected = %#v, got %#v", tt.expectedTarget, target)

				return
			}
		})
	}
}
