package idempotency

import (
	"bytes"
	"crypto/sha256"
	"io"
	"log"
	"net/http"
	"strings"
)

const defaultIdempotencyKeyHeader = "X-Idempotency-Key"

type config struct {
	store                Store
	idempotencyKeyHeader string
	whitelistedHeaders   []string
	scopeExtractorFn     func(r *http.Request) string
}

func newDefaultConfig() *config {
	return &config{
		store:                NewInMemStore(),
		idempotencyKeyHeader: defaultIdempotencyKeyHeader,
		whitelistedHeaders:   []string{"Content-Type"},
		scopeExtractorFn:     func(r *http.Request) string { return "" },
	}
}

// WithStore sets the store to use for idempotency.
func WithStore(store Store) func(*config) {
	return func(c *config) {
		c.store = store
	}
}

// WithIdempotencyKeyHeader sets the header to use for idempotency keys.
func WithIdempotencyKeyHeader(header string) func(*config) {
	return func(c *config) {
		c.idempotencyKeyHeader = header
	}
}

// WithWhitelistedHeaders sets the headers to include in the request signature.
func WithWhitelistedHeaders(headers ...string) func(*config) {
	return func(c *config) {
		c.whitelistedHeaders = headers
	}
}

// WithScopeExtractor sets a function to extract a scope from the request.
func WithScopeExtractor(fn func(r *http.Request) string) func(*config) {
	return func(c *config) {
		c.scopeExtractorFn = fn
	}
}

// Middleware enforces idempotency on non-GET requests.
// Requires X-Idempotency-Key for those methods.
func NewMiddleware(options ...func(*config)) func(http.Handler) http.Handler {
	conf := newDefaultConfig()

	for _, opt := range options {
		opt(conf)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For idempotency, we typically skip read-only methods:
			if r.Method == http.MethodGet || r.Method == http.MethodHead {
				next.ServeHTTP(w, r)
				return
			}

			// Check for the presence of the X-Idempotency-Key.
			key := r.Header.Get(conf.idempotencyKeyHeader)
			if key == "" {
				http.Error(w, "Missing `"+conf.idempotencyKeyHeader+"` header", http.StatusBadRequest)
				return
			}

			// Build a signature from the request body + relevant headers.
			requestSignature, err := buildRequestSignature(r, conf.whitelistedHeaders, conf.scopeExtractorFn)
			if err != nil {
				log.Printf("Error reading request body: %v", err)
				http.Error(w, "Could not read request body", http.StatusInternalServerError)
				return
			}

			// Check if there's a stored response for this key.
			ctx := r.Context()
			if resp, ok, err := conf.store.GetStoredResponse(ctx, key); err == nil && ok {
				// If we have a stored response, verify the request signature matches.
				if !bytes.Equal(resp.RequestSignature, requestSignature) {
					http.Error(w, "Idempotency-Key used with different request payload", http.StatusBadRequest)
					return
				}
				// If signature matches, replay the stored response.
				replayResponse(w, resp)
				return
			} else if err != nil {
				log.Printf("Error retrieving stored response: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Not completed. Check if in flight.
			if sig, ok, err := conf.store.GetInFlightSignature(ctx, key); err == nil && ok {
				// If in-flight, check if the request signature matches.
				if !bytes.Equal(sig, requestSignature) {
					http.Error(w, "Idempotency-Key used with different request payload", http.StatusBadRequest)
					return
				}
				// If signature matches, return 409 to let the client know the request is still in flight.
				http.Error(w, "Request still in-flight", http.StatusConflict)
				return
			} else if err != nil {
				log.Printf("Error retrieving in-flight signature: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// If not in-flight, mark as in-flight now.
			if err := conf.store.InsertInFlight(ctx, key, requestSignature); err != nil {
				// Could be a race condition if something got inserted concurrently,
				// or a store error. Return 409 or 500 as appropriate.
				log.Printf("Error inserting in-flight: %v", err)
				http.Error(w, "Conflict or internal error", http.StatusConflict)
				return
			}

			// Wrap the ResponseWriter so we can “tee” the response, storing it after the handler finishes.
			tw := newTeeResponseWriter(w)
			defer func() {
				// If there's a panic or something that prevents us from storing the final response,
				// we might want to remove the in-flight marker. Depends on your use-case.
				if rcv := recover(); rcv != nil {
					// Remove in-flight marker to allow a retry.
					_ = conf.store.RemoveInFlight(ctx, key)
					// Re-panic so that upper layers can handle the panic.
					panic(rcv)
				}
			}()

			// Call the actual handler
			next.ServeHTTP(tw, r)

			// Now we mark the request as completed with the stored response
			err = conf.store.MarkComplete(ctx, key, &StoredResponse{
				StatusCode:       tw.statusCode,
				Headers:          tw.header(),
				Body:             tw.body.Bytes(),
				RequestSignature: requestSignature,
			})
			if err != nil {
				log.Printf("Failed storing final response: %v", err)
				// Possibly remove in-flight marker so the client can retry.
				_ = conf.store.RemoveInFlight(ctx, key)
			}
		})
	}
}

// buildRequestSignature is an example function that reads the request body
// and optionally includes some headers for a “payload fingerprint.”
func buildRequestSignature(
	r *http.Request,
	whitelistedHeaders []string,
	scopeExtractorFn func(r *http.Request) string,
) ([]byte, error) {
	var buf bytes.Buffer

	// Copy the body so we can reuse it after hashing
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()

	// Put the body back into the request for the next handler
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Write the body into the buffer to incorporate it into the hash
	buf.Write(bodyBytes)

	// Optionally add some headers if you want them in the signature
	// For instance, content-type or a specific custom header
	for _, h := range whitelistedHeaders {
		if v := r.Header.Get(h); v != "" {
			buf.WriteString(h)
			buf.WriteString(v)
		}
	}

	// Optionally add a scope to the signature
	if scope := scopeExtractorFn(r); scope != "" {
		buf.WriteString(scope)
	}

	// Compute a sha256 hash of the combined data
	hash := sha256.Sum256(buf.Bytes())

	return hash[:], nil
}

// replayResponse writes a previously stored response to a ResponseWriter
func replayResponse(w http.ResponseWriter, resp *StoredResponse) {
	// Copy stored headers
	for k, vs := range resp.Headers {
		// We skip Content-Length because we might re-write it or let
		// http do so. Or you can do w.Header().Set("Content-Length", strconv.Itoa(len(resp.Body))).
		if strings.ToLower(k) == "content-length" {
			continue
		}
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}

// teeResponseWriter is a custom ResponseWriter that buffers the response
// while also passing writes through to the underlying ResponseWriter.
type teeResponseWriter struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func newTeeResponseWriter(w http.ResponseWriter) *teeResponseWriter {
	return &teeResponseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK, // Default
	}
}

// WriteHeader captures the status code, then calls the original WriteHeader
func (tw *teeResponseWriter) WriteHeader(code int) {
	tw.statusCode = code
	tw.ResponseWriter.WriteHeader(code)
}

// Write copies the data into our buffer, then passes it on
func (tw *teeResponseWriter) Write(data []byte) (int, error) {
	tw.body.Write(data)
	return tw.ResponseWriter.Write(data)
}

// header returns the final response headers at the time this function is called.
func (tw *teeResponseWriter) header() http.Header {
	// We access the real underlying writer’s header
	return tw.ResponseWriter.Header()
}
