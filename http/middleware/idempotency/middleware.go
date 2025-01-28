package idempotency

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

// Middleware enforces idempotency on non-GET requests.
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

			key := strings.TrimSpace(req.Header.Get(conf.idempotencyKeyHeader))
			if key == "" {
				if conf.idempotencyKeyIsOptional {
					next.ServeHTTP(respW, req)

					return
				}

				conf.errorToHTTPFn(respW, req,
					MissingIdempotencyKeyHeaderError{
						RequestContext{
							KeyHeader: conf.idempotencyKeyHeader,
						},
					},
				)

				return
			}

			requestHash, errS := buildRequestHash(conf.fingerprinterFn, req)
			if errS != nil {
				conf.errorToHTTPFn(respW, req, errS)

				return
			}

			if isFound := handleRequestWithIdempotency(
				conf,
				store,
				key,
				requestHash,
				respW,
				req,
			); isFound {
				return
			}

			// Try to lock the key to prevent concurrent requests
			newCtx, unlock, errL := store.TryLock(req.Context(), key)
			if errL != nil {
				conf.errorToHTTPFn(respW, req,
					RequestInFlightError{
						RequestContext{
							KeyHeader: conf.idempotencyKeyHeader,
							Key:       key,
						},
					},
				)

				return
			}

			defer unlock()

			// update the request context with the new context
			req = req.WithContext(newCtx)

			teeRespW := newTeeResponseWriter(respW)

			next.ServeHTTP(teeRespW, req)

			//nolint:contextcheck // req.Context() is a valid value
			if errSR := store.StoreResponse(req.Context(), key,
				&StoredResponse{
					StatusCode:  teeRespW.statusCode,
					Header:      teeRespW.header(),
					Body:        teeRespW.body.Bytes(),
					RequestHash: requestHash,
				},
			); errSR != nil {
				conf.errorToHTTPFn(respW, req, StoreResponseError{
					RequestContext: RequestContext{
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
	key string,
	requestHash []byte,
	respW http.ResponseWriter,
	req *http.Request,
) bool {
	ctx := req.Context()

	resp, exists, err := store.GetStoredResponse(ctx, key)
	if err != nil {
		conf.errorToHTTPFn(respW, req, err)

		return true
	}

	if exists {
		if !bytes.Equal(resp.RequestHash, requestHash) {
			conf.errorToHTTPFn(respW, req,
				MismatchedSignatureError{
					RequestContext{
						KeyHeader: conf.idempotencyKeyHeader,
						Key:       key,
					},
				},
			)

			return true
		}

		replayResponse(conf, respW, resp)

		return true
	}

	return false
}

// buildRequestHash is the function that will take the request
//
// and compute its hash
func buildRequestHash(fingerprinter func(*http.Request) ([]byte, error), req *http.Request) ([]byte, error) {
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
	// Copy stored headers
	for hdr, values := range resp.Header {
		// We skip Content-Length because we might re-write it or let
		// http do so. Or you can do w.Header().Set("Content-Length", strconv.Itoa(len(resp.Body))).
		if strings.EqualFold(hdr, "content-length") {
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
			conf.errorToHTTPFn(respW, nil, fmt.Errorf("failed writing replayed response body: %w", errW))
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
	return tw.ResponseWriter.Header()
}
