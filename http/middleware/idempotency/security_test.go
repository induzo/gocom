package idempotency

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware_InvalidKeyValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		key            string
		expectedStatus int
	}{
		{
			name:           "valid key accepted",
			key:            "valid-key-123",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "key with spaces rejected",
			key:            "invalid key",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "key with special chars rejected",
			key:            "invalid@key",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "too long key rejected",
			key:            strings.Repeat("a", 256),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := NewInMemStore()
			defer store.Close()

			middleware := NewMiddleware(store)

			handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("success"))
			}))

			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			req.Header.Set(DefaultIdempotencyKeyHeader, tt.key)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}
		})
	}
}

func TestMiddleware_CrossEndpointProtection(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	middleware := NewMiddleware(store)

	// Handler for endpoint 1
	handler1 := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("endpoint1-response"))
	}))

	// Handler for endpoint 2
	handler2 := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("endpoint2-response"))
	}))

	sameKey := "shared-key-123"

	// First request to endpoint 1
	req1 := httptest.NewRequest(http.MethodPost, "/api/endpoint1", nil)
	req1.Header.Set(DefaultIdempotencyKeyHeader, sameKey)

	rec1 := httptest.NewRecorder()
	handler1.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("First request failed: %d", rec1.Code)
	}

	body1 := rec1.Body.String()
	if body1 != "endpoint1-response" {
		t.Fatalf("Unexpected response: %s", body1)
	}

	// Second request to different endpoint with SAME key
	req2 := httptest.NewRequest(http.MethodPost, "/api/endpoint2", nil)
	req2.Header.Set(DefaultIdempotencyKeyHeader, sameKey)

	rec2 := httptest.NewRecorder()
	handler2.ServeHTTP(rec2, req2)

	body2 := rec2.Body.String()

	// Should NOT get endpoint1's response
	if body2 == "endpoint1-response" {
		t.Error(
			"SECURITY ISSUE: Same key on different endpoint returned cached response from different endpoint!",
		)
	}

	// Should get fresh response for endpoint2
	if body2 != "endpoint2-response" {
		t.Errorf("Expected fresh response for endpoint2, got: %s", body2)
	}
}

func TestMiddleware_CrossUserProtection(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	// Middleware with user extraction
	middleware := NewMiddleware(store, WithUserIDExtractor(func(r *http.Request) string {
		return r.Header.Get("X-User-Id")
	}))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-Id")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response-for-" + userID))
	}))

	sameKey := "shared-key-789"

	// Request from user1
	req1 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req1.Header.Set(DefaultIdempotencyKeyHeader, sameKey)
	req1.Header.Set("X-User-Id", "user1")

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("User1 request failed: %d", rec1.Code)
	}

	body1 := rec1.Body.String()

	// Request from user2 with SAME key
	req2 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req2.Header.Set(DefaultIdempotencyKeyHeader, sameKey)
	req2.Header.Set("X-User-Id", "user2")

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	body2 := rec2.Body.String()

	// Should NOT get user1's response
	if body2 == body1 {
		t.Error(
			"SECURITY ISSUE: Same idempotency key for different users returned cached response from another user!",
		)
	}

	// Should get fresh response for user2
	if !strings.Contains(body2, "user2") {
		t.Errorf("Expected response for user2, got: %s", body2)
	}
}

func TestMiddleware_HeaderSanitization(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	// Configure with limited allowed headers
	middleware := NewMiddleware(store, WithAllowedReplayHeaders("Content-Type"))

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Malicious-Header", "malicious-value")
		w.Header().Set("Set-Cookie", "session=hijack")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	}))

	key := "test-key-sanitize"

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req1.Header.Set(DefaultIdempotencyKeyHeader, key)

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	// Second request (replay)
	req2 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req2.Header.Set(DefaultIdempotencyKeyHeader, key)

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	// Verify malicious headers are NOT replayed
	if rec2.Header().Get("X-Malicious-Header") != "" {
		t.Error("SECURITY ISSUE: Malicious header was replayed!")
	}

	if rec2.Header().Get("Set-Cookie") != "" {
		t.Error("SECURITY ISSUE: Set-Cookie header was replayed!")
	}

	// Verify allowed header IS replayed
	if rec2.Header().Get("Content-Type") != "application/json" {
		t.Error("Expected Content-Type to be replayed")
	}

	// Verify replay header is set
	if rec2.Header().Get(DefaultIdempotentReplayedResponseHeader) != "true" {
		t.Error("Expected X-Idempotent-Replayed header to be set")
	}
}

func TestMiddleware_TTLExpiration(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	store.SetResponseTTL(100 * time.Millisecond)

	middleware := NewMiddleware(store)

	counter := 0
	handler := middleware(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { //nolint:revive // r not used in test
				counter++

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(string(rune('0' + counter))))
			},
		),
	)

	key := "test-key-ttl"

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req1.Header.Set(DefaultIdempotencyKeyHeader, key)

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	body1 := rec1.Body.String()

	// Second request immediately (should be cached)
	req2 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req2.Header.Set(DefaultIdempotencyKeyHeader, key)

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	body2 := rec2.Body.String()

	if body1 != body2 {
		t.Errorf("Expected cached response, got different: %s vs %s", body1, body2)
	}

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Third request after TTL (should be fresh)
	req3 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req3.Header.Set(DefaultIdempotencyKeyHeader, key)

	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	body3 := rec3.Body.String()

	if body1 == body3 {
		t.Error("Expected fresh response after TTL expiration")
	}
}

func TestMiddleware_LockTimeout(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	store.lockTimeout = 200 * time.Millisecond

	middleware := NewMiddleware(store)

	slowHandler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond) // Longer than lock timeout
		w.WriteHeader(http.StatusOK)
	}))

	key := "test-key-lock"

	// Start first slow request
	req1 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req1.Header.Set(DefaultIdempotencyKeyHeader, key)

	done := make(chan bool)

	go func() {
		rec1 := httptest.NewRecorder()
		slowHandler.ServeHTTP(rec1, req1)

		done <- true
	}()

	// Wait a bit for lock to be acquired
	time.Sleep(50 * time.Millisecond)

	// Second request should be blocked initially
	req2 := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req2.Header.Set(DefaultIdempotencyKeyHeader, key)

	rec2 := httptest.NewRecorder()
	slowHandler.ServeHTTP(rec2, req2)

	// Should get conflict error
	if rec2.Code != http.StatusConflict {
		t.Errorf("Expected 409 Conflict during lock, got %d", rec2.Code)
	}

	<-done // Wait for first request to complete
}

func TestMiddleware_CrossMethodProtection(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	middleware := NewMiddleware(store, WithAffectedMethods("POST", "PUT"))

	handlerPOST := middleware(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { //nolint:revive // r not used in test
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("POST-response"))
			},
		),
	)

	handlerPUT := middleware(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) { //nolint:revive // r not used in test
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("PUT-response"))
			},
		),
	)

	sameKey := "cross-method-key"

	// POST request
	req1 := httptest.NewRequest(http.MethodPost, "/api/resource", nil)
	req1.Header.Set(DefaultIdempotencyKeyHeader, sameKey)

	rec1 := httptest.NewRecorder()
	handlerPOST.ServeHTTP(rec1, req1)

	body1 := rec1.Body.String()
	if body1 != "POST-response" {
		t.Fatalf("Expected POST-response, got: %s", body1)
	}

	// PUT request with same key (different method)
	req2 := httptest.NewRequest(http.MethodPut, "/api/resource", nil)
	req2.Header.Set(DefaultIdempotencyKeyHeader, sameKey)

	rec2 := httptest.NewRecorder()
	handlerPUT.ServeHTTP(rec2, req2)

	body2 := rec2.Body.String()

	// Should NOT get POST's response for PUT request
	if body2 == "POST-response" {
		t.Error(
			"SECURITY ISSUE: Same key with different method returned cached response from different method!",
		)
	}

	if body2 != "PUT-response" {
		t.Errorf("Expected fresh PUT response, got: %s", body2)
	}
}

func TestMiddleware_PayloadMismatchDetection(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	middleware := NewMiddleware(store)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("processed: " + string(body)))
	}))

	key := "payload-test-key"

	// First request with payload1
	req1 := httptest.NewRequest(
		http.MethodPost,
		"/api/test",
		bytes.NewBufferString(`{"amount":100}`),
	)
	req1.Header.Set(DefaultIdempotencyKeyHeader, key)

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	// Second request with DIFFERENT payload but SAME key
	req2 := httptest.NewRequest(
		http.MethodPost,
		"/api/test",
		bytes.NewBufferString(`{"amount":999}`),
	)
	req2.Header.Set(DefaultIdempotencyKeyHeader, key)

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	// Should detect payload mismatch - expect 409 Conflict or 422 Unprocessable Entity
	if rec2.Code != http.StatusConflict && rec2.Code != http.StatusUnprocessableEntity {
		t.Errorf("Expected error status for payload mismatch, got %d", rec2.Code)
	}

	// Should NOT process different payload
	if strings.Contains(rec2.Body.String(), "999") {
		t.Error("SECURITY ISSUE: Different payload was processed with same key!")
	}
}

func TestMiddleware_ContextKeyAvailability(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	middleware := NewMiddleware(store)

	keyFound := false

	var extractedKey string

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key, ok := r.Context().Value(IdempotencyKeyCtxKey).(string); ok {
			keyFound = true
			extractedKey = key
		}

		w.WriteHeader(http.StatusOK)
	}))

	expectedKey := "context-test-key"

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set(DefaultIdempotencyKeyHeader, expectedKey)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !keyFound {
		t.Error("Idempotency key not found in request context")
	}

	if extractedKey != expectedKey {
		t.Errorf("Expected key %s in context, got %s", expectedKey, extractedKey)
	}
}
