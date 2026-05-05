package idempotency

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// FuzzValidateIdempotencyKey explores the validator's input space and
// pins the contract: any key it accepts must contain only documented
// characters, be within the length cap, and never contain the
// store-key delimiter ':'.
func FuzzValidateIdempotencyKey(f *testing.F) {
	seeds := []string{
		"",
		"a",
		"abc-123",
		"key.with.dots",
		"key with spaces",
		"user:POST:/p:k",
		"a@b",
		strings.Repeat("a", 255),
		strings.Repeat("a", 256),
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, key string) {
		err := validateIdempotencyKey(key)
		if err != nil {
			return
		}

		if len(key) == 0 || len(key) > maxKeyLength {
			t.Errorf("validator accepted out-of-range key %q (len=%d)", key, len(key))
		}

		if !validKeyPattern.MatchString(key) {
			t.Errorf("validator accepted key %q that fails the charset pattern", key)
		}

		if strings.ContainsRune(key, ':') {
			t.Errorf("validator accepted key %q containing the store-key delimiter ':'", key)
		}
	})
}

// FuzzBuildStoreKey checks that distinct user IDs produce distinct store
// keys for the same idempotency key, method, and path.
func FuzzBuildStoreKey(f *testing.F) {
	f.Add("user1", "user2", "/api/pay", "valid-key")
	f.Add("alice", "bob", "/orders", "k1")

	f.Fuzz(func(t *testing.T, userA, userB, path, key string) {
		if userA == userB || userA == "" || userB == "" {
			t.Skip()
		}

		if !strings.HasPrefix(path, "/") {
			t.Skip()
		}

		if validateIdempotencyKey(key) != nil {
			t.Skip()
		}

		req := httptest.NewRequest(http.MethodPost, path, nil)

		extract := func(uid string) UserIDExtractorFn {
			return func(*http.Request) string { return uid }
		}

		ka := buildStoreKey(req, key, extract(userA))
		kb := buildStoreKey(req, key, extract(userB))

		if ka == kb {
			t.Errorf("distinct users %q and %q produced the same store key %q", userA, userB, ka)
		}
	})
}
