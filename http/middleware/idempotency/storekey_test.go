package idempotency

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidateIdempotencyKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{
			name:    "valid alphanumeric key",
			key:     "abc123",
			wantErr: false,
		},
		{
			name:    "valid key with hyphen",
			key:     "key-123",
			wantErr: false,
		},
		{
			name:    "valid key with underscore",
			key:     "key_123",
			wantErr: false,
		},
		{
			name:    "valid key with period",
			key:     "key.123",
			wantErr: false,
		},
		{
			name:    "valid mixed",
			key:     "Key-123_abc.xyz",
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			wantErr: true,
		},
		{
			name:    "key with spaces",
			key:     "key with spaces",
			wantErr: true,
		},
		{
			name:    "key with special chars",
			key:     "key@123",
			wantErr: true,
		},
		{
			name:    "key with slash",
			key:     "key/123",
			wantErr: true,
		},
		{
			name:    "key with colon (injection attempt)",
			key:     "user:POST:/api/payment:malicious",
			wantErr: true,
		},
		{
			name:    "key too long",
			key:     strings.Repeat("a", 256),
			wantErr: true,
		},
		{
			name:    "key at max length",
			key:     strings.Repeat("a", 255),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateIdempotencyKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIdempotencyKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildStoreKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		method          string
		path            string
		idempotencyKey  string
		userID          string
		expectedContain []string
	}{
		{
			name:           "with user ID",
			method:         "POST",
			path:           "/api/payment",
			idempotencyKey: "key-123",
			userID:         "user-456",
			expectedContain: []string{
				"user-456",
				"POST",
				"/api/payment",
				"key-123",
			},
		},
		{
			name:           "without user ID",
			method:         "POST",
			path:           "/api/transfer",
			idempotencyKey: "key-789",
			userID:         "",
			expectedContain: []string{
				"POST",
				"/api/transfer",
				"key-789",
			},
		},
		{
			name:           "path normalization (lowercase)",
			method:         "POST",
			path:           "/API/Payment",
			idempotencyKey: "key-abc",
			userID:         "user1",
			expectedContain: []string{
				"user1",
				"POST",
				"/api/payment", // Should be lowercased
				"key-abc",
			},
		},
		{
			name:           "method normalization (uppercase)",
			method:         "post",
			path:           "/api/test",
			idempotencyKey: "key-xyz",
			userID:         "",
			expectedContain: []string{
				"POST", // Should be uppercased
				"/api/test",
				"key-xyz",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tt.method, tt.path, nil)

			var userExtractor UserIDExtractorFn
			if tt.userID != "" {
				userExtractor = func(*http.Request) string {
					return tt.userID
				}
			}

			result := buildStoreKey(req, tt.idempotencyKey, userExtractor)

			// Verify all expected parts are in the key
			for _, expected := range tt.expectedContain {
				if !strings.Contains(result, expected) {
					t.Errorf("buildStoreKey() = %v, should contain %v", result, expected)
				}
			}

			// Verify proper delimiter usage
			if !strings.Contains(result, ":") {
				t.Error("buildStoreKey() should use ':' as delimiter")
			}
		})
	}
}

func TestBuildStoreKey_PreventsCrossEndpointAttack(t *testing.T) {
	t.Parallel()

	userExtractor := func(*http.Request) string { return "user123" }
	idempotencyKey := "same-key"

	// Same key on different endpoints should produce different store keys
	req1 := httptest.NewRequest(http.MethodPost, "/api/payment", nil)
	req2 := httptest.NewRequest(http.MethodPost, "/api/transfer", nil)

	key1 := buildStoreKey(req1, idempotencyKey, userExtractor)
	key2 := buildStoreKey(req2, idempotencyKey, userExtractor)

	if key1 == key2 {
		t.Errorf(
			"Same idempotency key on different endpoints should produce different store keys: %v == %v",
			key1,
			key2,
		)
	}
}

func TestBuildStoreKey_PreventsCrossMethodAttack(t *testing.T) {
	t.Parallel()

	userExtractor := func(*http.Request) string { return "user123" }
	idempotencyKey := "same-key"

	// Same key with different methods should produce different store keys
	req1 := httptest.NewRequest(http.MethodPost, "/api/resource", nil)
	req2 := httptest.NewRequest(http.MethodPut, "/api/resource", nil)

	key1 := buildStoreKey(req1, idempotencyKey, userExtractor)
	key2 := buildStoreKey(req2, idempotencyKey, userExtractor)

	if key1 == key2 {
		t.Errorf(
			"Same idempotency key with different methods should produce different store keys: %v == %v",
			key1,
			key2,
		)
	}
}

func TestBuildStoreKey_PreventsCrossUserAttack(t *testing.T) {
	t.Parallel()

	idempotencyKey := "same-key"
	path := "/api/payment"

	req1 := httptest.NewRequest(http.MethodPost, path, nil)
	req2 := httptest.NewRequest(http.MethodPost, path, nil)

	// Different users should produce different store keys
	userExtractor1 := func(*http.Request) string { return "user1" }
	userExtractor2 := func(*http.Request) string { return "user2" }

	key1 := buildStoreKey(req1, idempotencyKey, userExtractor1)
	key2 := buildStoreKey(req2, idempotencyKey, userExtractor2)

	if key1 == key2 {
		t.Errorf(
			"Same idempotency key for different users should produce different store keys: %v == %v",
			key1,
			key2,
		)
	}
}
