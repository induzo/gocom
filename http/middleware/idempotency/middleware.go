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

			key := req.Header.Get(conf.idempotencyKeyHeader)
			if key == "" {
				if conf.IdempotencyKeyIsOptional {
					next.ServeHTTP(respW, req)

					return
				}

				handleMissingKey(conf, respW, req)

				return
			}

			requestSignature, err := buildRequestSignature(req, conf.whitelistedHeaders, conf.scopeExtractorFn)
			if err != nil {
				conf.errorToHTTPFn(conf.logger, respW, req, fmt.Errorf("buildRequestSignature: %w", err))

				return
			}

			if handleStoredResponse(conf, store, respW, req, key, requestSignature) {
				return
			}

			if handleInFlightRequest(conf, store, respW, req, key, requestSignature) {
				return
			}

			if err := store.InsertInFlight(req.Context(), key, requestSignature); err != nil {
				conf.errorToHTTPFn(conf.logger, respW, req, fmt.Errorf("InsertInFlight: %w", err))

				return
			}

			teeRespW := newTeeResponseWriter(respW)

			defer handlePanic(conf, req, key)

			next.ServeHTTP(teeRespW, req)

			markRequestComplete(conf, store, req, key, teeRespW, requestSignature)
		})
	}
}

func isReadOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func handleMissingKey(conf *config, respW http.ResponseWriter, req *http.Request) {
	conf.errorToHTTPFn(
		conf.logger, respW, req,
		MissingIdempotencyKeyHeaderError{
			ExpectedHeader: conf.idempotencyKeyHeader,
			Method:         req.Method,
			URL:            req.URL.String(),
		},
	)
}

func handleStoredResponse(
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
		conf.errorToHTTPFn(conf.logger, respW, req, err)

		return true
	}

	if exists {
		if !bytes.Equal(resp.RequestSignature, requestSignature) {
			conf.errorToHTTPFn(
				conf.logger, respW, req,
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

func handleInFlightRequest(
	conf *config,
	store Store,
	respW http.ResponseWriter,
	req *http.Request,
	key string,
	requestSignature []byte,
) bool {
	ctx := req.Context()

	sig, exists, err := store.GetInFlightSignature(ctx, key)
	if err != nil {
		conf.errorToHTTPFn(conf.logger, respW, req, fmt.Errorf("GetInFlightSignature: %w", err))

		return true
	}

	if exists {
		if !bytes.Equal(sig, requestSignature) {
			conf.errorToHTTPFn(
				conf.logger, respW, req,
				MismatchedSignatureError{
					Key:    key,
					Method: req.Method,
					URL:    req.URL.String(),
				},
			)

			return true
		}

		conf.errorToHTTPFn(
			conf.logger, respW, req,
			RequestStillInFlightError{
				Key:    key,
				Method: req.Method,
				URL:    req.URL.String(),
			},
		)

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

func markRequestComplete(
	conf *config,
	store Store,
	req *http.Request,
	key string,
	teeRespW *teeResponseWriter,
	requestSignature []byte,
) {
	err := store.MarkComplete(req.Context(), key, &StoredResponse{
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

// buildRequestSignature is an example function that reads the request body
// and optionally includes some headers for a “payload fingerprint.”
func buildRequestSignature(
	req *http.Request,
	whitelistedHeaders []string,
	scopeExtractorFn func(r *http.Request) string,
) ([]byte, error) {
	var buf bytes.Buffer

	// Copy the body so we can reuse it after hashing
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("buildRequestSignature: %w", err)
	}

	defer req.Body.Close()

	// Put the body back into the request for the next handler
	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Write the body into the buffer to incorporate it into the hash
	buf.Write(bodyBytes)

	// Optionally add some headers if you want them in the signature
	// For instance, content-type or a specific custom header
	for _, h := range whitelistedHeaders {
		if v := req.Header.Get(h); v != "" {
			buf.WriteString(h)
			buf.WriteString(v)
		}
	}

	// Optionally add a scope to the signature
	if scope := scopeExtractorFn(req); scope != "" {
		buf.WriteString(scope)
	}

	// Compute a sha256 hash of the combined data
	hash := sha256.Sum256(buf.Bytes())

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
