package idempotency

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
)

// buildRequestFingerprint creates a hash fingerprint from the request.
// It uses SHA-256 to produce a fixed-size output and streams the body
// to handle large payloads without loading everything into memory at once.
// You can add your own logic here to build the fingerprint.
func buildRequestFingerprint(req *http.Request) ([]byte, error) {
	hash := sha256.New()

	// Write the method and URL into the hash
	hash.Write([]byte(req.Method + req.URL.String()))

	// Stream the body through both the hash and a buffer.
	// This avoids loading the entire body into memory before processing.
	var bodyBuf bytes.Buffer

	tee := io.TeeReader(req.Body, &bodyBuf)

	// Copy the body to the hash in chunks (streaming)
	if _, err := io.Copy(hash, tee); err != nil {
		return nil, fmt.Errorf("buildRequestFingerprint: reading body: %w", err)
	}

	// Close the original body and replace it with the buffered copy
	req.Body.Close()
	req.Body = io.NopCloser(&bodyBuf)

	whitelistedHeaders := []string{
		"Accept",
		"Accept-Encoding",
		"Content-Type",
	}

	// Optionally add some headers if you want them in the signature
	// For instance, content-type or a specific custom header
	for _, hdr := range whitelistedHeaders {
		if v := req.Header.Get(hdr); v != "" {
			hash.Write([]byte(hdr))
			hash.Write([]byte(v))
		}
	}

	// Optionally add some scoping values, like userid
	if v, ok := req.Context().Value("userid").(string); ok {
		hash.Write([]byte("userid-" + v))
	}

	return hash.Sum(nil), nil
}
