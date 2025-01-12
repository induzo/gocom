package idempotency

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

// Middleware enforces idempotency on non-GET requests.
func NewMiddleware(store Store, options ...func(*config)) func(http.Handler) http.Handler {
	conf := newDefaultConfig()

	for _, opt := range options {
		opt(conf)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
			if isReadOnlyMethod(req.Method) {
				next.ServeHTTP(respW, req)

				return
			}

			key := strings.TrimSpace(req.Header.Get(conf.idempotencyKeyHeader))
			if key == "" {
				if conf.idempotencyKeyIsOptional {
					next.ServeHTTP(respW, req)

					return
				}

				handleMissingKey(conf, respW, req)

				return
			}

			requestHash, errS := buildRequestHash(conf.fingerprinterFn, req)
			if errS != nil {
				conf.errorToHTTPFn(conf.logger, respW, req, key, errS)

				return
			}

			if isFound := handleRequestWithIdempotency(conf, store, respW, req, key, requestHash); isFound {
				return
			}

			// Try to lock the key to prevent concurrent requests
			newCtx, unlock, errL := store.TryLock(req.Context(), key)
			if errL != nil {
				conf.errorToHTTPFn(conf.logger, respW, req, key, RequestInFlightError{
					Key:    key,
					Method: req.Method,
					URL:    req.URL.String(),
				})

				return
			}

			defer unlock()

			// update the request context with the new context
			req = req.WithContext(newCtx)

			teeRespW := newTeeResponseWriter(respW)

			defer handlePanic(conf, req, key)

			next.ServeHTTP(teeRespW, req)

			storeResponse(conf, store, req, key, teeRespW, requestHash)
		})
	}
}

func isReadOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func handleMissingKey(conf *config, respW http.ResponseWriter, req *http.Request) {
	conf.errorToHTTPFn(
		conf.logger, respW, req, conf.idempotencyKeyHeader,
		MissingIdempotencyKeyHeaderError{
			ExpectedHeader: conf.idempotencyKeyHeader,
			Method:         req.Method,
			URL:            req.URL.String(),
		},
	)
}

func handleRequestWithIdempotency(
	conf *config,
	store Store,
	respW http.ResponseWriter,
	req *http.Request,
	key string,
	requestSignature []byte,
) bool {
	ctx := req.Context()

	resp, exists, err := store.GetStoredResponse(ctx, key)
	if err != nil {
		conf.errorToHTTPFn(conf.logger, respW, req, key, err)

		return true
	}

	if exists {
		if !bytes.Equal(resp.RequestSignature, requestSignature) {
			conf.errorToHTTPFn(
				conf.logger, respW, req, conf.idempotencyKeyHeader,
				MismatchedSignatureError{
					Key:    key,
					Method: req.Method,
					URL:    req.URL.String(),
				},
			)

			return true
		}

		replayResponse(conf.logger, respW, resp)

		return true
	}

	return false
}

func handlePanic(conf *config, req *http.Request, key string) {
	if rcv := recover(); rcv != nil {
		conf.logger.Error(
			"failed storing final response: panicked!",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.String("idempotency-key", key),
			slog.Any("err", rcv),
		)
	}
}

func storeResponse(
	conf *config,
	store Store,
	req *http.Request,
	key string,
	teeRespW *teeResponseWriter,
	requestSignature []byte,
) {
	err := store.StoreResponse(req.Context(), key, &StoredResponse{
		StatusCode:       teeRespW.statusCode,
		Headers:          teeRespW.header(),
		Body:             teeRespW.body.Bytes(),
		RequestSignature: requestSignature,
	})
	if err != nil {
		conf.logger.Error(
			"failed storing final response",
			slog.String("method", req.Method),
			slog.String("url", req.URL.String()),
			slog.String("idempotency-key", key),
			slog.Any("err", err),
		)
	}
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
func replayResponse(logger *slog.Logger, respW http.ResponseWriter, resp *StoredResponse) {
	// Copy stored headers
	for hdr, values := range resp.Headers {
		// We skip Content-Length because we might re-write it or let
		// http do so. Or you can do w.Header().Set("Content-Length", strconv.Itoa(len(resp.Body))).
		if strings.EqualFold(hdr, "content-length") {
			continue
		}

		for _, v := range values {
			respW.Header().Add(hdr, v)
		}
	}

	respW.WriteHeader(resp.StatusCode)

	if len(resp.Body) > 0 {
		if _, errW := respW.Write(resp.Body); errW != nil {
			logger.Error(
				"failed writing response body",
				slog.Any("err", errW),
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
	return tw.ResponseWriter.Header()
}
