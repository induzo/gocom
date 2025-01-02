package idempotency

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// Middleware enforces idempotency on non-GET requests.
// Requires X-Idempotency-Key for those methods.
func NewMiddleware(store Store, options ...func(*config)) func(http.Handler) http.Handler {
	conf := newDefaultConfig()

	for _, opt := range options {
		opt(conf)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// For idempotency, we typically skip read-only methods:
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)

				return
			}

			// Check for the presence of the X-Idempotency-Key.
			key := r.Header.Get(conf.idempotencyKeyHeader)
			if key == "" {
				if conf.IdempotencyKeyIsOptional {
					next.ServeHTTP(w, r)

					return
				}

				conf.errorToHTTPFn(
					w, r,
					MissingIdempotencyKeyHeaderError{
						ExpectedHeader: conf.idempotencyKeyHeader,
						Method:         r.Method,
						URL:            r.URL.String(),
					},
				)

				return
			}

			// Build a signature from the request body + relevant headers.
			requestSignature, errB := buildRequestSignature(r, conf.whitelistedHeaders, conf.scopeExtractorFn)
			if errB != nil {
				conf.errorToHTTPFn(w, r, fmt.Errorf("buildRequestSignature: %v", errB))

				return
			}

			// Check if there's a stored response for this key.
			ctx := r.Context()
			resp, ok, errG := store.GetStoredResponse(ctx, key)
			if errG != nil {
				conf.errorToHTTPFn(w, r, errG)

				return
			}

			if ok {
				// If we have a stored response, verify the request signature matches.
				if !bytes.Equal(resp.RequestSignature, requestSignature) {
					conf.errorToHTTPFn(
						w, r,
						MismatchedSignatureError{
							Key:    key,
							Method: r.Method,
							URL:    r.URL.String(),
						},
					)

					return
				}

				// If signature matches, replay the stored response.
				replayResponse(w, resp)

				return
			}

			// Not completed. Check if in flight.
			sig, ok, errGin := store.GetInFlightSignature(ctx, key)
			if errGin != nil {
				conf.errorToHTTPFn(w, r, fmt.Errorf("GetInFlightSignature: %v", errGin))

				return
			}

			if ok {
				// If in-flight, check if the request signature matches.
				if !bytes.Equal(sig, requestSignature) {
					conf.errorToHTTPFn(
						w, r,
						MismatchedSignatureError{
							Key:    key,
							Method: r.Method,
							URL:    r.URL.String(),
						},
					)

					return
				}

				conf.errorToHTTPFn(
					w, r,
					RequestStillInFlightError{
						Key:    key,
						Method: r.Method,
						URL:    r.URL.String(),
						Sig:    string(sig),
					},
				)

				return
			}

			// If not in-flight, mark as in-flight now.
			if err := store.InsertInFlight(ctx, key, requestSignature); err != nil {
				conf.errorToHTTPFn(w, r, fmt.Errorf("InsertInFlight: %v", err))

				return
			}

			// Wrap the ResponseWriter so we can “tee” the response, storing it after the handler finishes.
			tw := newTeeResponseWriter(w)
			defer func() {
				// If there's a panic or something that prevents us from storing the final response,
				// we want to remove the in-flight marker.
				if rcv := recover(); rcv != nil {
					slog.Error(
						"failed storing final response: panicked!",
						slog.String("method", r.Method),
						slog.String("url", r.URL.String()),
						slog.String("idempotency-key", key),
						slog.Any("err", rcv),
					)
				}
			}()

			// Call the actual handler
			next.ServeHTTP(tw, r)

			// Now we mark the request as completed with the stored response
			errMC := store.MarkComplete(ctx, key, &StoredResponse{
				StatusCode:       tw.statusCode,
				Headers:          tw.header(),
				Body:             tw.body.Bytes(),
				RequestSignature: requestSignature,
			})
			if errMC != nil {
				slog.Error(
					"failed storing final response",
					slog.String("method", r.Method),
					slog.String("url", r.URL.String()),
					slog.String("idempotency-key", key),
					slog.Any("err", errMC),
				)
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
