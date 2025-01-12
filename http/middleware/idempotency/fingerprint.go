package idempotency

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

// This is a sample fingerprinter function that hashes the request body and some headers
// You can add your own logic here to build the fingerprint
func buildRequestFingerprint(req *http.Request) ([]byte, error) {
	var buf bytes.Buffer

	// Copy the body so we can reuse it after hashing
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("buildRequestFingerprint: %w", err)
	}

	defer req.Body.Close()

	// write the method and URL into the buffer
	buf.WriteString(req.Method + req.URL.String())

	// Put the body back into the request for the next handler
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Write the body into the buffer to incorporate it into the hash
	buf.Write(bodyBytes)

	whitelistedHeaders := []string{
		"Accept",
		"Accept-Encoding",
		"Content-Type",
	}

	// Optionally add some headers if you want them in the signature
	// For instance, content-type or a specific custom header
	for _, h := range whitelistedHeaders {
		if v := req.Header.Get(h); v != "" {
			buf.WriteString(h)
			buf.WriteString(v)
		}
	}

	// Optionally add some scoping values, like userid
	if v, ok := req.Context().Value("userid").(string); ok {
		buf.WriteString("userid-" + v)
	}

	return buf.Bytes(), nil
}
