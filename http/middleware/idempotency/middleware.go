package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// Middleware enforces idempotency on non-GET requests.
//
//nolint:cyclop,gocognit // Complexity is acceptable for middleware validation logic.
func NewMiddleware(store Store, options ...Option) func(http.Handler) http.Handler {
	conf := newDefaultConfig()

	for _, opt := range options {
		opt(conf)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
			if !slices.Contains(conf.affectedMethods, strings.ToUpper(req.Method)) {
				next.ServeHTTP(respW, req)

				return
			}

			if slices.Contains(conf.ignoredURLPaths, strings.ToLower(req.URL.Path)) {
				next.ServeHTTP(respW, req)

				return
			}

			endExtractKey := conf.tracerFn(req, "idempotency.extract_key")
			key := strings.TrimSpace(req.Header.Get(conf.idempotencyKeyHeader))

			endExtractKey()

			if key == "" {
				if conf.idempotencyKeyIsOptional {
					next.ServeHTTP(respW, req)

					return
				}

				conf.errorToHTTPFn(respW, req,
					MissingIdempotencyKeyHeaderError{
						RequestContext{
							URL:       req.URL.String(),
							Method:    req.Method,
							Key:       key,
							KeyHeader: conf.idempotencyKeyHeader,
						},
					},
				)

				return
			}

			// Validate the idempotency key
			endValidateKey := conf.tracerFn(req, "idempotency.validate_key")
			err := validateIdempotencyKey(key)

			endValidateKey()

			if err != nil {
				conf.errorToHTTPFn(respW, req,
					InvalidIdempotencyKeyError{
						URL:       req.URL.String(),
						Method:    req.Method,
						Key:       key,
						KeyHeader: conf.idempotencyKeyHeader,
						Err:       err,
					},
				)

				return
			}

			// Build composite store key (user:method:path:key)
			endBuildStoreKey := conf.tracerFn(req, "idempotency.build_store_key")
			storeKey := buildStoreKey(req, key, conf.userIDExtractor)

			endBuildStoreKey()

			// set key in the request context
			req = req.WithContext(
				context.WithValue(req.Context(), IdempotencyKeyCtxKey, key),
			)

			endBuildHash := conf.tracerFn(req, "idempotency.build_request_hash")
			requestHash, errS := buildRequestHash(conf.fingerprinterFn, req)

			endBuildHash()

			if errS != nil {
				conf.errorToHTTPFn(respW, req, errS)

				return
			}

			endCheckStored := conf.tracerFn(req, "idempotency.handle_response")
			isFound := handleRequestWithIdempotency(
				conf,
				store,
				storeKey,
				requestHash,
				respW,
				req,
				key,
			)

			endCheckStored()

			if isFound {
				return
			}

			// Try to lock the key to prevent concurrent requests
			endLock := conf.tracerFn(req, "idempotency.try_lock")

			lockCtx, unlock, errL := store.TryLock(req.Context(), storeKey)
			if errL != nil {
				conf.errorToHTTPFn(respW, req,
					RequestInFlightError{
						RequestContext{
							URL:       req.URL.String(),
							Method:    req.Method,
							KeyHeader: conf.idempotencyKeyHeader,
							Key:       key,
						},
					},
				)

				return
			}

			if lockCtx == nil {
				lockCtx = req.Context()
			}

			endLock()

			defer func() {
				// Try to lock the key to prevent concurrent requests
				endUnlock := conf.tracerFn(req, "idempotency.unlock")

				unlock()
				endUnlock()
			}()

			teeRespW := newTeeResponseWriter(respW, conf.maxResponseBodyBytes)

			next.ServeHTTP(teeRespW, req)

			if teeRespW.overflowed {
				// The response exceeded the cacheable size and was streamed
				// straight to the client. There is nothing complete to store,
				// so leave the key unstored; a retry re-executes the handler.
				return
			}

			endStore := conf.tracerFn(req, "idempotency.store_response")
			errSR := store.StoreResponse(lockCtx, storeKey,
				&StoredResponse{
					StatusCode:  teeRespW.statusCode,
					Header:      teeRespW.header().Clone(),
					Body:        teeRespW.body.Bytes(),
					RequestHash: requestHash,
				},
			)

			endStore()

			if errSR != nil {
				conf.errorToHTTPFn(respW, req, StoreResponseError{
					URL:       req.URL.String(),
					Method:    req.Method,
					KeyHeader: conf.idempotencyKeyHeader,
					Key:       key,
					Err:       errSR,
				})

				return
			}
		})
	}
}

func handleRequestWithIdempotency(
	conf *config,
	store Store,
	storeKey string,
	requestHash []byte,
	respW http.ResponseWriter,
	req *http.Request,
	originalKey string,
) bool {
	resp, exists, err := store.GetStoredResponse(req.Context(), storeKey)
	if err != nil {
		conf.errorToHTTPFn(respW, req, GetStoredResponseError{
			URL:       req.URL.String(),
			Method:    req.Method,
			KeyHeader: conf.idempotencyKeyHeader,
			Key:       originalKey,
			Err:       err,
		})

		return true
	}

	if exists {
		if !bytes.Equal(resp.RequestHash, requestHash) {
			conf.errorToHTTPFn(respW, req,
				MismatchedSignatureError{
					RequestContext{
						URL:       req.URL.String(),
						Method:    req.Method,
						KeyHeader: conf.idempotencyKeyHeader,
						Key:       originalKey,
					},
				},
			)

			return true
		}

		endReplay := conf.tracerFn(req, "idempotency.replay_response")

		replayResponse(conf, respW, req, resp)
		endReplay()

		return true
	}

	return false
}

type ContextKey string

const IdempotencyKeyCtxKey ContextKey = "idempotency_key"

// buildRequestHash is the function that will take the request
//
// and compute its hash
func buildRequestHash(
	fingerprinter func(*http.Request) ([]byte, error),
	req *http.Request,
) ([]byte, error) {
	// Compute the request fingerprint
	fingerprint, err := fingerprinter(req)
	if err != nil {
		return nil, fmt.Errorf("failed to compute request fingerprint: %w", err)
	}

	// Compute a sha256 hash of the combined data
	hash := sha256.Sum256(fingerprint)

	return hash[:], nil
}

// replayResponse writes a previously stored response to a ResponseWriter.
func replayResponse(
	conf *config,
	respW http.ResponseWriter,
	req *http.Request,
	resp *StoredResponse,
) {
	// Create a map of allowed headers for fast lookup
	allowedHeaders := make(map[string]bool)
	for _, hdr := range conf.allowedReplayHeaders {
		allowedHeaders[strings.ToLower(hdr)] = true
	}

	// Copy only safe/allowed stored headers
	for hdr, values := range resp.Header {
		lowerHdr := strings.ToLower(hdr)

		// Skip Content-Length (will be set automatically) and disallowed headers
		if lowerHdr == "content-length" || !allowedHeaders[lowerHdr] {
			continue
		}

		for _, v := range values {
			respW.Header().Add(hdr, v)
		}
	}

	respW.Header().Add(conf.idempotentReplayedHeader, "true")

	respW.WriteHeader(resp.StatusCode)

	if len(resp.Body) > 0 {
		if _, errW := respW.Write(resp.Body); errW != nil {
			conf.errorToHTTPFn(
				respW,
				req,
				fmt.Errorf("failed writing replayed response body: %w", errW),
			)
		}
	}
}

// teeResponseWriter is a custom ResponseWriter that buffers the response
// while also passing writes through to the underlying ResponseWriter.
type teeResponseWriter struct {
	http.ResponseWriter
	body         *bytes.Buffer
	statusCode   int
	maxBodyBytes int64
	overflowed   bool
}

func newTeeResponseWriter(w http.ResponseWriter, maxBodyBytes int64) *teeResponseWriter {
	return &teeResponseWriter{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK, // Default
		maxBodyBytes:   maxBodyBytes,
	}
}

// WriteHeader captures the status code, then calls the original WriteHeader
func (tw *teeResponseWriter) WriteHeader(code int) {
	tw.statusCode = code
	tw.ResponseWriter.WriteHeader(code)
}

// Write copies the data into our buffer (up to maxBodyBytes), then passes it
// on to the underlying writer. Once the buffered body would exceed
// maxBodyBytes the tee stops buffering, drops what it has, and flags the
// response as overflowed so the caller skips storing it; the client still
// receives every byte. The returned count reflects the bytes written to the
// underlying writer, per the io.Writer contract.
func (tw *teeResponseWriter) Write(data []byte) (int, error) {
	if !tw.overflowed {
		if tw.maxBodyBytes > 0 &&
			int64(tw.body.Len())+int64(len(data)) > tw.maxBodyBytes {
			tw.overflowed = true
			tw.body.Reset()
		} else {
			// bytes.Buffer.Write only ever returns a nil error.
			_, _ = tw.body.Write(data)
		}
	}

	writtenBytesCount, errWR := tw.ResponseWriter.Write(data)
	if errWR != nil {
		return writtenBytesCount, fmt.Errorf("teeResponseWriter ResponseWriter Write: %w", errWR)
	}

	return writtenBytesCount, nil
}

// Unwrap exposes the underlying ResponseWriter so that http.ResponseController
// (and direct optional-interface assertions) can reach capabilities such as
// http.Flusher, http.Hijacker, and SetWriteDeadline that the tee does not
// implement itself.
func (tw *teeResponseWriter) Unwrap() http.ResponseWriter {
	return tw.ResponseWriter
}

// header returns the final response headers at the time this function is called.
func (tw *teeResponseWriter) header() http.Header {
	// We access the real underlying writer’s header
	return tw.Header()
}
