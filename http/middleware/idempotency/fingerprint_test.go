package idempotency

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testFingerprintMax int64 = 1024 * 1024

// Test_buildRequestFingerprint_Deterministic asserts the fingerprint is
// stable for identical inputs.
func Test_buildRequestFingerprint_Deterministic(t *testing.T) {
	t.Parallel()

	body := "{\"amount\":100}"

	build := func() []byte {
		req := httptest.NewRequest(
			http.MethodPost,
			"http://example.com/api/pay",
			strings.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")

		got, err := buildRequestFingerprint(req, testFingerprintMax)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		return got
	}

	a := build()

	b := build()
	if !bytes.Equal(a, b) {
		t.Errorf("identical requests produced different fingerprints: %x vs %x", a, b)
	}
}

// Test_buildRequestFingerprint_DistinctForDistinctInputs ensures that
// changes to method, path, query, headers, body, or user ID produce
// distinct hashes.
func Test_buildRequestFingerprint_DistinctForDistinctInputs(t *testing.T) {
	t.Parallel()

	makeReq := func(opts ...func(*http.Request)) *http.Request {
		req := httptest.NewRequest(
			http.MethodPost,
			"http://example.com/api/pay",
			strings.NewReader("body"),
		)
		req.Header.Set("Content-Type", "application/json")

		for _, opt := range opts {
			opt(req)
		}

		return req
	}

	base, err := buildRequestFingerprint(makeReq(), testFingerprintMax)
	if err != nil {
		t.Fatalf("base fingerprint: %v", err)
	}

	cases := map[string]func(*http.Request){
		"different method": func(r *http.Request) { r.Method = http.MethodPut },
		"different path": func(r *http.Request) {
			r.URL.Path = "/api/refund"
		},
		"different query": func(r *http.Request) {
			r.URL.RawQuery = "x=1"
		},
		"different body": func(r *http.Request) {
			r.Body = io.NopCloser(strings.NewReader("different body"))
		},
		"different content-type": func(r *http.Request) {
			r.Header.Set("Content-Type", "application/xml")
		},
		"different user id (typed key)": func(r *http.Request) {
			ctx := context.WithValue(r.Context(), UserIDCtxKey, "vincent")
			*r = *r.WithContext(ctx)
		},
	}

	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := buildRequestFingerprint(makeReq(mod), testFingerprintMax)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}

			if bytes.Equal(got, base) {
				t.Errorf("%s did not change the fingerprint", name)
			}
		})
	}
}

// Test_buildRequestFingerprint_PathCanonicalization ensures that path
// case differences are normalized so they hash identically; this matches
// buildStoreKey's lowercasing.
func Test_buildRequestFingerprint_PathCanonicalization(t *testing.T) {
	t.Parallel()

	req1 := httptest.NewRequest(
		http.MethodPost,
		"http://example.com/API/Pay",
		strings.NewReader("b"),
	)
	req1.Header.Set("Content-Type", "application/json")

	req2 := httptest.NewRequest(
		http.MethodPost,
		"http://example.com/api/pay",
		strings.NewReader("b"),
	)
	req2.Header.Set("Content-Type", "application/json")

	a, err := buildRequestFingerprint(req1, testFingerprintMax)
	if err != nil {
		t.Fatalf("a: %v", err)
	}

	b, err := buildRequestFingerprint(req2, testFingerprintMax)
	if err != nil {
		t.Fatalf("b: %v", err)
	}

	if !bytes.Equal(a, b) {
		t.Errorf("path-case difference yielded different fingerprints: %x vs %x", a, b)
	}
}

// Test_buildRequestFingerprint_LegacyUserIDKey verifies the historical
// untyped string "userid" context value is still honored as a fallback.
func Test_buildRequestFingerprint_LegacyUserIDKey(t *testing.T) {
	t.Parallel()

	//nolint:revive,staticcheck // legacy fallback under test
	ctx := context.WithValue(context.Background(), userIDCtxKeyLegacy, "vincent")
	req := httptest.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"http://example.com/api/pay",
		strings.NewReader("b"),
	)
	req.Header.Set("Content-Type", "application/json")

	withLegacy, err := buildRequestFingerprint(req, testFingerprintMax)
	if err != nil {
		t.Fatalf("withLegacy: %v", err)
	}

	plainReq := httptest.NewRequest(
		http.MethodPost,
		"http://example.com/api/pay",
		strings.NewReader("b"),
	)
	plainReq.Header.Set("Content-Type", "application/json")

	plain, err := buildRequestFingerprint(plainReq, testFingerprintMax)
	if err != nil {
		t.Fatalf("plain: %v", err)
	}

	if bytes.Equal(withLegacy, plain) {
		t.Errorf("legacy userid context value did not affect fingerprint")
	}
}

// Test_buildRequestFingerprint_BodyCap verifies that bodies above the
// configured limit produce BodyTooLargeError.
func Test_buildRequestFingerprint_BodyCap(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("a", 2048)
	req := httptest.NewRequest(
		http.MethodPost,
		"http://example.com/upload",
		strings.NewReader(body),
	)

	const limit int64 = 1024

	_, err := buildRequestFingerprint(req, limit)

	var btl BodyTooLargeError
	if !errors.As(err, &btl) {
		t.Fatalf("expected BodyTooLargeError, got %T (%v)", err, err)
	}

	if btl.Limit != limit {
		t.Errorf("Limit = %d, want %d", btl.Limit, limit)
	}
}

// Test_buildRequestFingerprint_BodyExactlyAtLimitOK verifies a body that
// is exactly maxBodyBytes is accepted (the limit is inclusive).
func Test_buildRequestFingerprint_BodyExactlyAtLimitOK(t *testing.T) {
	t.Parallel()

	const limit int64 = 64

	body := strings.Repeat("a", int(limit))
	req := httptest.NewRequest(
		http.MethodPost,
		"http://example.com/upload",
		strings.NewReader(body),
	)

	if _, err := buildRequestFingerprint(req, limit); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test_buildRequestFingerprint_BodyReusable confirms the body is restored
// after fingerprinting so downstream handlers can still read it.
func Test_buildRequestFingerprint_BodyReusable(t *testing.T) {
	t.Parallel()

	body := "reusable body"
	req := httptest.NewRequest(http.MethodPost, "http://example.com/api", strings.NewReader(body))

	if _, err := buildRequestFingerprint(req, testFingerprintMax); err != nil {
		t.Fatalf("fingerprint: %v", err)
	}

	got, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	if string(got) != body {
		t.Errorf("body after fingerprint = %q, want %q", got, body)
	}
}
