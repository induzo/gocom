package idempotency

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"strings"
)

// errBodyTooLarge is the underlying error wrapped by BodyTooLargeError.
var errBodyTooLarge = errors.New("request body exceeds idempotency fingerprint limit")

// buildRequestFingerprint hashes a canonical representation of the request
// (method, lowered path, query, body, whitelisted headers, user ID) using
// SHA-256. Each piece is length-prefixed so distinct fields cannot collide
// at concatenation boundaries.
//
// maxBodyBytes bounds the body bytes consumed; requests above the limit
// return a BodyTooLargeError so the middleware can reject them with HTTP 413.
func buildRequestFingerprint(req *http.Request, maxBodyBytes int64) ([]byte, error) {
	hasher := sha256.New()

	writePrefixed(hasher, []byte(strings.ToUpper(req.Method)))
	writePrefixed(hasher, []byte(strings.ToLower(req.URL.Path)))
	writePrefixed(hasher, []byte(req.URL.RawQuery))

	bodyBytes, err := readLimitedBody(req, maxBodyBytes)
	if err != nil {
		return nil, err
	}

	writePrefixed(hasher, bodyBytes)

	whitelistedHeaders := []string{
		"Accept",
		"Accept-Encoding",
		headerContentType,
	}

	for _, hdr := range whitelistedHeaders {
		if v := req.Header.Get(hdr); v != "" {
			writePrefixed(hasher, []byte(hdr))
			writePrefixed(hasher, []byte(v))
		}
	}

	if v := userIDFromContext(req); v != "" {
		writePrefixed(hasher, []byte("userid-"+v))
	}

	return hasher.Sum(nil), nil
}

// writePrefixed writes a 4-byte big-endian length followed by payload, so
// distinct fields cannot collide at the boundary.
func writePrefixed(hasher hash.Hash, payload []byte) {
	var sz [4]byte

	// payloads larger than 4 GiB are rejected by the body cap upstream.
	binary.BigEndian.PutUint32(sz[:], uint32(len(payload))) //nolint:gosec // bounded

	_, _ = hasher.Write(sz[:])
	_, _ = hasher.Write(payload)
}

// readLimitedBody reads up to maxBodyBytes from req.Body and re-attaches a
// fresh reader so downstream handlers still see the body. If the body is
// larger than the limit it returns BodyTooLargeError.
func readLimitedBody(req *http.Request, maxBodyBytes int64) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}

	limited := io.LimitReader(req.Body, maxBodyBytes+1)

	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("buildRequestFingerprint: reading body: %w", err)
	}

	if int64(len(buf)) > maxBodyBytes {
		_ = req.Body.Close()

		return nil, BodyTooLargeError{
			Limit: maxBodyBytes,
			Err:   errBodyTooLarge,
		}
	}

	_ = req.Body.Close()
	req.Body = io.NopCloser(bytes.NewReader(buf))

	return buf, nil
}

// userIDFromContext mirrors defaultUserIDExtractor's lookup so the default
// fingerprinter and the default extractor agree on what constitutes the
// scoping user ID.
func userIDFromContext(req *http.Request) string {
	if v, ok := req.Context().Value(UserIDCtxKey).(string); ok {
		return v
	}

	if v, ok := req.Context().Value(userIDCtxKeyLegacy).(string); ok {
		return v
	}

	return ""
}
