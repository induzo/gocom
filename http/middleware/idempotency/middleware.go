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
//nolint:cyclop // Complexity is acceptable for middleware validation logic.
func NewMiddleware(store Store, options ...Option) func(http.Handler) http.Handler {
	conf := newDefaultConfig()

	for _, opt := range options {
		opt(conf)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
			if !slices.Contains(conf.affectedMethods, req.Method) {
				next.ServeHTTP(respW, req)

				return
			}

			if slices.Contains(conf.ignoredURLPaths, strings.ToLower(req.URL.Path)) {
				next.ServeHTTP(respW, req)

				return
			}

			ctx, endExtractKey := conf.tracerFn(req, "idempotency.extract_key")
			req = req.WithContext(ctx)
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
			ctx, endValidateKey := conf.tracerFn(req, "idempotency.validate_key")
			req = req.WithContext(ctx)
			err := validateIdempotencyKey(key)

			endValidateKey()

			if err != nil {
				conf.errorToHTTPFn(respW, req,
					InvalidIdempotencyKeyError{
						RequestContext: RequestContext{
							URL:       req.URL.String(),
							Method:    req.Method,
							Key:       key,
							KeyHeader: conf.idempotencyKeyHeader,
						},
						Err: err,
					},
				)

				return
			}

			// Build composite store key (user:method:path:key)
			ctx, endBuildStoreKey := conf.tracerFn(req, "idempotency.build_store_key")
			req = req.WithContext(ctx)
			storeKey := buildStoreKey(req, key, conf.userIDExtractor)

			endBuildStoreKey()

			// set key in the request context
			req = req.WithContext(
				context.WithValue(req.Context(), IdempotencyKeyCtxKey, key),
			)

			ctx, endBuildHash := conf.tracerFn(req, "idempotency.build_request_hash")
			req = req.WithContext(ctx)
			requestHash, errS := buildRequestHash(conf.fingerprinterFn, req)

			endBuildHash()

			if errS != nil {
				conf.errorToHTTPFn(respW, req, errS)

				return
			}

			ctx, endCheckStored := conf.tracerFn(req, "idempotency.check_stored_response")
			req = req.WithContext(ctx)
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
			ctx, endLock := conf.tracerFn(req, "idempotency.lock")
			req = req.WithContext(ctx)
			newCtx, unlock, errL := store.TryLock(req.Context(), storeKey)

			endLock()

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

			defer unlock()

			// update the request context with the new context
			//nolint:contextcheck // newCtx is derived from req.Context() in TryLock
			req = req.WithContext(newCtx)

			teeRespW := newTeeResponseWriter(respW)

			next.ServeHTTP(teeRespW, req)

			ctx, endStore := conf.tracerFn(req, "idempotency.store_response")
			req = req.WithContext(ctx)
			//nolint:contextcheck // req.Context() is updated with newCtx above
			errSR := store.StoreResponse(req.Context(), storeKey,
				&StoredResponse{
					StatusCode:  teeRespW.statusCode,
					Header:      teeRespW.header(),
					Body:        teeRespW.body.Bytes(),
					RequestHash: requestHash,
				},
			)

			endStore()

			if errSR != nil {
				conf.errorToHTTPFn(respW, req, StoreResponseError{
					RequestContext: RequestContext{
						URL:       req.URL.String(),
						Method:    req.Method,
						KeyHeader: conf.idempotencyKeyHeader,
						Key:       key,
					},
					Err: errSR,
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
	ctx, endGetStored := conf.tracerFn(req, "idempotency.get_stored_response")
	req = req.WithContext(ctx)
	resp, exists, err := store.GetStoredResponse(req.Context(), storeKey)

	endGetStored()

	if err != nil {
		conf.errorToHTTPFn(respW, req, err)

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

		ctx, endReplay := conf.tracerFn(req, "idempotency.replay_response")
		req = req.WithContext(ctx)
		replayResponse(conf, respW, resp)
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

// replayResponse writes a previously stored response to a ResponseWriter
func replayResponse(conf *config, respW http.ResponseWriter, resp *StoredResponse) {
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
				nil,
				fmt.Errorf("failed writing replayed response body: %w", errW),
			)
		}
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
	if _, errW := tw.body.Write(data); errW != nil {
		return 0, fmt.Errorf("teeResponseWriter body Write: %w", errW)
	}

	writtenBytesCount, errWR := tw.ResponseWriter.Write(data)
	if errWR != nil {
		return 0, fmt.Errorf("teeResponseWriter ResponseWriter Write: %w", errWR)
	}

	return writtenBytesCount, nil
}

// header returns the final response headers at the time this function is called.
func (tw *teeResponseWriter) header() http.Header {
	// We access the real underlying writer’s header
	return tw.Header()
}
