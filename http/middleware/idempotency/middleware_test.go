package idempotency

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewMiddleware(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	idempotencyMiddleware := NewMiddleware(store)

	if idempotencyMiddleware == nil {
		t.Error("NewMiddleware returned nil")
	}
}

func TestMiddleware_ServeHTTP(t *testing.T) {
	t.Parallel()

	type req struct {
		urlPath string
		method  string
		key     string
		startAt time.Duration
		body    string
	}

	type resp struct {
		key    string
		status int
		body   string
	}

	testc := []struct {
		name                             string
		reqProcessingTime                time.Duration
		reqws                            []req
		options                          []Option
		withFaultyStoreResponseStore     bool
		withFaultyGetStoredResponseStore bool
		expectedResp                     map[int]resp
		expectedCounter                  int
	}{
		{
			name:              "1 request",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "onekey",
					startAt: 0,
					body:    "hola",
				},
			},
			options: nil,
			expectedResp: map[int]resp{
				0: {
					key:    "onekey",
					status: http.StatusOK,
					body:   "hola",
				},
			},
			expectedCounter: 1,
		},
		{
			name:              "1 request, missing idempot header",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					startAt: 0,
					body:    "hola",
				},
			},
			options: nil,
			expectedResp: map[int]resp{
				0: {
					key:    "",
					status: http.StatusBadRequest,
					body:   "MissingIdempotencyKeyHeaderError",
				},
			},
			expectedCounter: 0,
		},
		{
			name:              "1 request, missing idempot header, but optional",
			reqProcessingTime: 0,
			reqws: []req{
				{
					method:  http.MethodPost,
					startAt: 0,
					body:    "hola",
				},
			},
			options: []Option{WithOptionalIdempotencyKey()},
			expectedResp: map[int]resp{
				0: {
					key:    "onekey",
					status: http.StatusOK,
					body:   "hola",
				},
			},
			expectedCounter: 1,
		},
		{
			name:              "2 concurrent requests",
			reqProcessingTime: 100 * time.Millisecond,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "samekey",
					startAt: 0,
					body:    "hola",
				},
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "samekey",
					startAt: 50 * time.Millisecond,
					body:    "hola",
				},
			},
			options: nil,
			expectedResp: map[int]resp{
				0: {
					key:    "samekey",
					status: http.StatusOK,
					body:   "hola",
				},
				1: {
					key:    "samekey",
					status: http.StatusConflict,
					body:   "RequestInFlightError",
				},
			},
			expectedCounter: 1,
		},
		{
			name:              "2 requests, 1 after the other",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "samekey",
					startAt: 0,
					body:    "hola",
				},
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "samekey",
					startAt: 20 * time.Millisecond,
					body:    "hola",
				},
			},
			options: nil,
			expectedResp: map[int]resp{
				0: {
					key:    "samekey",
					status: http.StatusOK,
					body:   "hola",
				},
				1: {
					key:    "samekey",
					status: http.StatusOK,
					body:   "hola",
				},
			},
			expectedCounter: 1,
		},
		{
			name:              "2 totally diff requests",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "firstkey",
					startAt: 0,
					body:    "hola",
				},
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "secondkey",
					startAt: 20 * time.Millisecond,
					body:    "hola",
				},
			},
			options: nil,
			expectedResp: map[int]resp{
				0: {
					key:    "firstkey",
					status: http.StatusOK,
					body:   "hola",
				},
				1: {
					key:    "secondkey",
					status: http.StatusOK,
					body:   "hola",
				},
			},
			expectedCounter: 2,
		},
		{
			name:              "get request",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodGet,
					key:     "getkey",
					startAt: 0,
					body:    "hola",
				},
				{
					urlPath: "test",
					method:  http.MethodGet,
					key:     "getkey",
					startAt: 0,
					body:    "hola",
				},
			},
			options: nil,
			expectedResp: map[int]resp{
				0: {
					key:    "getkey",
					status: http.StatusOK,
					body:   "hola",
				},
				1: {
					key:    "getkey",
					status: http.StatusOK,
					body:   "hola",
				},
			},
			expectedCounter: 2,
		},
		{
			name:              "request on an ignored URL",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "/ignoredurl",
					method:  http.MethodPost,
					key:     "postkey",
					startAt: 0,
					body:    "hola",
				},
				{
					urlPath: "/ignoredurl",
					method:  http.MethodPost,
					key:     "postkey",
					startAt: 0,
					body:    "hola",
				},
			},
			options: []Option{
				WithIgnoredURLPaths("/ignoredurl"),
			},
			expectedResp: map[int]resp{
				0: {
					key:    "getkey",
					status: http.StatusOK,
					body:   "hola",
				},
				1: {
					key:    "getkey",
					status: http.StatusOK,
					body:   "hola",
				},
			},
			expectedCounter: 2,
		},
		{
			name:              "1 request with failing fingerprinter",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "onekey",
					startAt: 0,
					body:    "hola",
				},
			},
			options: []Option{
				WithFingerprinter(
					func(_ *http.Request) ([]byte, error) {
						return nil, errors.New("fingerprinter error")
					},
				),
			},
			expectedResp: map[int]resp{
				0: {
					key:    "onekey",
					status: http.StatusInternalServerError,
					body:   "internal server error",
				},
			},
			expectedCounter: 0,
		},
		{
			name:              "1 request that exists in store but with diff fingerprint",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "onekey",
					startAt: 0,
					body:    "hola",
				},
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "onekey",
					startAt: 10 * time.Millisecond,
					body:    "hola",
				},
			},
			options: []Option{
				WithFingerprinter(
					func(_ *http.Request) ([]byte, error) {
						return []byte(time.Now().Format(time.RFC3339Nano)), nil
					},
				),
			},
			expectedResp: map[int]resp{
				0: {
					key:    "onekey",
					status: http.StatusOK,
					body:   "hola",
				},
				1: {
					key:    "onekey",
					status: http.StatusBadRequest,
					body:   "MismatchedSignatureError",
				},
			},
			expectedCounter: 1,
		},
		{
			name:              "1 request with faulty store response issue",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "faultystorekey",
					startAt: 0,
					body:    "hola",
				},
			},
			options:                      nil,
			withFaultyStoreResponseStore: true,
			expectedResp: map[int]resp{
				0: {
					key:    "onekey",
					status: http.StatusOK,
					body:   "holaStoreResponseError: error storing response: StoreResponse: store error",
				},
			},
			expectedCounter: 1,
		},
		{
			name:              "1 requests, with error getting stored response",
			reqProcessingTime: 0,
			reqws: []req{
				{
					urlPath: "test",
					method:  http.MethodPost,
					key:     "samekey",
					startAt: 0,
					body:    "hola",
				},
			},
			options:                          nil,
			withFaultyGetStoredResponseStore: true,
			expectedResp: map[int]resp{
				0: {
					key:    "samekey",
					status: http.StatusInternalServerError,
					body:   "internal server error: error getting stored response: GetStoredResponse: get stored response error",
				},
			},
			expectedCounter: 0,
		},
	}

	for _, tt := range testc {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			options := append([]Option{WithErrorToHTTPFn(errorToString)}, tt.options...)

			store := NewInMemStore()
			t.Cleanup(store.Close)

			if tt.withFaultyStoreResponseStore {
				store.withActiveStoreResponseError()
			}

			if tt.withFaultyGetStoredResponseStore {
				store.withActiveGetStoredResponseError()
			}

			idempotencyMiddleware := NewMiddleware(store, options...)

			mux := http.NewServeMux()

			counter := int32(0)

			mux.Handle("/",
				idempotencyMiddleware(
					http.HandlerFunc(func(respW http.ResponseWriter, reqw *http.Request) {
						time.Sleep(tt.reqProcessingTime)

						atomic.AddInt32(&counter, 1)

						bdy, _ := io.ReadAll(reqw.Body)

						respW.Write(bdy)
					}),
				),
			)

			server := httptest.NewServer(mux)
			defer server.Close()

			var wg sync.WaitGroup

			for reqIdx, reqw := range tt.reqws {
				wg.Add(1)

				go func(id int, key, body string, startAt time.Duration) {
					defer wg.Done()

					time.Sleep(startAt)

					status, body, err := sendReq(
						context.Background(),
						reqw.method,
						reqw.urlPath,
						server,
						key,
						body,
					)
					if err != nil {
						t.Errorf("SendPOSTReq: %v", err)
					}

					if _, ok := tt.expectedResp[id]; !ok {
						t.Errorf("response id %d not found", id)

						return
					}

					if tt.expectedResp[id].status != status {
						t.Errorf("expected status %d, got %d", tt.expectedResp[id].status, status)
					}

					if tt.expectedResp[id].body != strings.TrimSpace(body) {
						t.Errorf("expected body `%s`, got `%s`", tt.expectedResp[id].body, body)
					}
				}(reqIdx, reqw.key, reqw.body, reqw.startAt)
			}

			wg.Wait()

			if int(counter) != tt.expectedCounter {
				t.Errorf("expected counter %d, got %d", tt.expectedCounter, counter)
			}
		})
	}
}

func TestReplayResponse(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	storedResp := &StoredResponse{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":   []string{"application/json"},
			"Content-Length": []string{"4"},
			"X-Test":         []string{"test"},
		},
		Body: []byte("body"),
	}

	store.responses.Store("key", storedResp)

	respRec := httptest.NewRecorder()

	replayResponse(&config{
		idempotentReplayedHeader: DefaultIdempotentReplayedResponseHeader,
		errorToHTTPFn:            errorToString,
		allowedReplayHeaders:     []string{"Content-Type", "X-Test"},
	}, respRec, httptest.NewRequest(http.MethodPost, "/replay", nil), storedResp)

	resp := respRec.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)

	if !bytes.Equal(respBody, storedResp.Body) {
		t.Errorf("expected body `%s`, got `%s`", storedResp.Body, resp.Body)
	}

	if len(resp.Header) != len(storedResp.Header) {
		t.Errorf("expected header len %d, got %d", len(storedResp.Header), len(resp.Header))
	}

	for k, v := range storedResp.Header {
		if k == "Content-Length" {
			continue
		}

		if !reflect.DeepEqual(resp.Header[k], v) {
			t.Errorf("expected header `%v`, got `%v`", v, resp.Header[k])
		}
	}
}

func TestTeeResponseWriterWriteHeader(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)

	tee := newTeeResponseWriter(httptest.NewRecorder(), DefaultMaxResponseBodyBytes)

	tee.WriteHeader(http.StatusOK)

	if tee.statusCode != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, tee.statusCode)
	}

	if buf.Len() != 0 {
		t.Errorf("expected buf len 0, got %d", buf.Len())
	}
}

func TestTeeResponseWriterWrite(t *testing.T) {
	t.Parallel()

	buf := httptest.NewRecorder()

	tee := newTeeResponseWriter(buf, DefaultMaxResponseBodyBytes)

	_, _ = tee.Write([]byte("hola"))

	if buf.Body.Len() != 4 {
		t.Errorf("expected buf len 4, got %d", buf.Body.Len())
	}
}

func sendReq(
	ctx context.Context,
	method, urlPath string,
	server *httptest.Server,
	key, reqBody string,
) (int, string, error) {
	if len(urlPath) > 0 && urlPath[0] != '/' {
		urlPath = "/" + urlPath
	}

	req, errR := http.NewRequestWithContext(
		ctx,
		method,
		server.URL+urlPath,
		bytes.NewBufferString(reqBody),
	)
	if errR != nil {
		return 0, "", errR
	}

	req.Header.Set(DefaultIdempotencyKeyHeader, key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	body, errB := io.ReadAll(resp.Body)
	if errB != nil {
		return 0, "", errB
	}

	return resp.StatusCode, string(body), nil
}

func TestMiddleware_IdempotencyKeyInContext(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	idempotencyMiddleware := NewMiddleware(store)

	expectedKey := "test-idempotency-key"

	var capturedKey string

	handler := idempotencyMiddleware(
		http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			// Capture the idempotency key from context
			key, ok := req.Context().Value(IdempotencyKeyCtxKey).(string)
			if ok {
				capturedKey = key
			}
		}),
	)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set(DefaultIdempotencyKeyHeader, expectedKey)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedKey != expectedKey {
		t.Errorf("expected idempotency key in context to be %q, got %q", expectedKey, capturedKey)
	}
}

func TestMiddleware_WithTracer(t *testing.T) {
	t.Parallel()

	t.Run("successful request creates all expected spans", func(t *testing.T) {
		t.Parallel()

		store := NewInMemStore()
		defer store.Close()

		var mu sync.Mutex

		spans := []string{}

		tracerFn := func(_ *http.Request, spanName string) func() {
			mu.Lock()

			spans = append(spans, spanName+":start")

			mu.Unlock()

			return func() {
				mu.Lock()

				spans = append(spans, spanName+":end")

				mu.Unlock()
			}
		}

		middleware := NewMiddleware(store, WithTracer(tracerFn))

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test response"))
		}))

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test body"))
		req.Header.Set(DefaultIdempotencyKeyHeader, "test-key-123")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Check that all expected spans were created
		// Spans now end immediately after their operation (except lock which uses defer)
		expectedSpans := []string{
			"idempotency.extract_key:start",
			"idempotency.extract_key:end",
			"idempotency.validate_key:start",
			"idempotency.validate_key:end",
			"idempotency.build_store_key:start",
			"idempotency.build_store_key:end",
			"idempotency.build_request_hash:start",
			"idempotency.build_request_hash:end",
			"idempotency.handle_response:start",
			"idempotency.handle_response:end",
			"idempotency.try_lock:start",
			"idempotency.try_lock:end",
			"idempotency.recheck_response:start",
			"idempotency.recheck_response:end",
			"idempotency.store_response:start",
			"idempotency.store_response:end",
			"idempotency.unlock:start",
			"idempotency.unlock:end",
		}

		mu.Lock()
		defer mu.Unlock()

		if len(spans) != len(expectedSpans) {
			t.Errorf("expected %d spans, got %d\nExpected: %v\nGot: %v",
				len(expectedSpans), len(spans), expectedSpans, spans)
		}

		for i, expected := range expectedSpans {
			if i >= len(spans) {
				break
			}

			if spans[i] != expected {
				t.Errorf("span[%d]: expected %q, got %q", i, expected, spans[i])
			}
		}
	})

	t.Run("replayed request creates replay spans", func(t *testing.T) {
		t.Parallel()

		store := NewInMemStore()
		defer store.Close()

		var mu sync.Mutex

		spans := []string{}

		tracerFn := func(_ *http.Request, spanName string) func() {
			mu.Lock()

			spans = append(spans, spanName)

			mu.Unlock()

			return func() {}
		}

		middleware := NewMiddleware(store, WithTracer(tracerFn))

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("first response"))
		}))

		// First request
		req1 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test body"))
		req1.Header.Set(DefaultIdempotencyKeyHeader, "replay-key")

		rec1 := httptest.NewRecorder()
		handler.ServeHTTP(rec1, req1)

		// Clear spans
		mu.Lock()

		spans = []string{}

		mu.Unlock()

		// Second request with same key - should be replayed
		req2 := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("test body"))
		req2.Header.Set(DefaultIdempotencyKeyHeader, "replay-key")

		rec2 := httptest.NewRecorder()
		handler.ServeHTTP(rec2, req2)

		mu.Lock()
		defer mu.Unlock()

		// Should have middleware, extract, validate, build_store_key, build_hash, check_stored, get_stored, and replay spans
		if !containsSpan(spans, "idempotency.handle_response") {
			t.Error("expected handle_response span")
		}

		if !containsSpan(spans, "idempotency.replay_response") {
			t.Error("expected replay_response span for cached response")
		}
		// Should NOT have execute_request or store_response since it's replayed
		if containsSpan(spans, "idempotency.execute_request") {
			t.Error("should not have execute_request span for replayed response")
		}

		if containsSpan(spans, "idempotency.store_response") {
			t.Error("should not have store_response span for replayed response")
		}
	})

	t.Run("missing key still creates tracing spans", func(t *testing.T) {
		t.Parallel()

		store := NewInMemStore()
		defer store.Close()

		var mu sync.Mutex

		spans := []string{}

		tracerFn := func(_ *http.Request, spanName string) func() {
			mu.Lock()

			spans = append(spans, spanName)

			mu.Unlock()

			return func() {}
		}

		middleware := NewMiddleware(store, WithTracer(tracerFn), WithErrorToHTTPFn(errorToString))

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		// No idempotency key header set

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		mu.Lock()
		defer mu.Unlock()

		// Should have middleware and extract_key spans even though request fails
		if !containsSpan(spans, "idempotency.") {
			t.Error("expected middleware span even for failed request")
		}

		if !containsSpan(spans, "idempotency.extract_key") {
			t.Error("expected extract_key span even for failed request")
		}
	})

	t.Run("no-op tracer by default", func(t *testing.T) {
		t.Parallel()

		store := NewInMemStore()
		defer store.Close()

		// Create middleware without WithTracer option
		middleware := NewMiddleware(store)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("test"))
		}))

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("body"))
		req.Header.Set(DefaultIdempotencyKeyHeader, "test-key")

		rec := httptest.NewRecorder()

		// Should not panic with nil tracer
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("concurrent requests both get traced", func(t *testing.T) {
		t.Parallel()

		store := NewInMemStore()
		defer store.Close()

		spanCalls := atomic.Int32{}

		tracerFn := func(_ *http.Request, _ string) func() {
			spanCalls.Add(1)
			return func() {}
		}

		middleware := NewMiddleware(store, WithTracer(tracerFn), WithErrorToHTTPFn(errorToString))

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(50 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}))

		var wg sync.WaitGroup
		wg.Add(2)

		// First request
		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("body"))
			req.Header.Set(DefaultIdempotencyKeyHeader, "concurrent-key")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}()

		// Second concurrent request with same key
		go func() {
			defer wg.Done()

			time.Sleep(10 * time.Millisecond)

			req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("body"))
			req.Header.Set(DefaultIdempotencyKeyHeader, "concurrent-key")

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}()

		wg.Wait()

		// Both requests should have created spans
		if spanCalls.Load() == 0 {
			t.Error("expected tracer to be called for concurrent requests")
		}
	})
}

// TestMiddleware_RequestInFlight_Deterministic exercises the in-flight
// path without timing-based sleeps: the first request blocks inside the
// handler on a barrier; while it is blocked the second request fires and
// must observe a 409 Conflict (RequestInFlightError) from the lock.
func TestMiddleware_RequestInFlight_Deterministic(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	t.Cleanup(store.Close)

	var (
		started = make(chan struct{}, 1)
		release = make(chan struct{})
	)

	handler := NewMiddleware(store, WithErrorToHTTPFn(errorToString))(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			select {
			case started <- struct{}{}:
			default:
			}

			<-release
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}))

	server := httptest.NewServer(handler)
	defer server.Close()

	const key = "deterministic-key"

	type result struct {
		status int
		body   string
		err    error
	}

	first := make(chan result, 1)

	go func() {
		s, b, err := sendReq(context.Background(), http.MethodPost, "/", server, key, "body")
		first <- result{status: s, body: b, err: err}
	}()

	<-started

	secondStatus, secondBody, err := sendReq(
		context.Background(),
		http.MethodPost,
		"/",
		server,
		key,
		"body",
	)
	if err != nil {
		t.Fatalf("second request: %v", err)
	}

	if secondStatus != http.StatusConflict {
		t.Errorf("second request status = %d, want %d", secondStatus, http.StatusConflict)
	}

	if !strings.Contains(secondBody, "RequestInFlightError") {
		t.Errorf("second request body = %q, want it to contain RequestInFlightError", secondBody)
	}

	close(release)

	r := <-first
	if r.err != nil {
		t.Fatalf("first request: %v", r.err)
	}

	if r.status != http.StatusOK {
		t.Errorf("first request status = %d, want %d", r.status, http.StatusOK)
	}
}

// containsSpan checks if a span name exists in the slice
func containsSpan(spans []string, spanName string) bool {
	for _, s := range spans {
		if strings.Contains(s, spanName) {
			return true
		}
	}

	return false
}

// errorToString write the error type returned
func errorToString(
	writer http.ResponseWriter,
	_ *http.Request,
	err error,
) {
	switch {
	case errors.As(err, &MissingIdempotencyKeyHeaderError{}):
		http.Error(writer, "MissingIdempotencyKeyHeaderError", http.StatusBadRequest)
	case errors.As(err, &RequestInFlightError{}):
		http.Error(writer, "RequestInFlightError", http.StatusConflict)
	case errors.As(err, &TryLockError{}):
		http.Error(writer, fmt.Sprintf("TryLockError: %v", err), http.StatusInternalServerError)
	case errors.As(err, &MismatchedSignatureError{}):
		http.Error(writer, "MismatchedSignatureError", http.StatusBadRequest)
	case errors.As(err, &StoreResponseError{}):
		http.Error(writer, fmt.Sprintf("StoreResponseError: %v", err), http.StatusOK)
	case errors.As(err, &GetStoredResponseError{}):
		http.Error(
			writer,
			fmt.Sprintf("internal server error: %v", err),
			http.StatusInternalServerError,
		)
	default:
		http.Error(
			writer,
			"internal server error",
			http.StatusInternalServerError,
		)
	}
}

// flushOnlyWriter wraps an http.ResponseWriter and exposes http.Flusher only
// through Unwrap (not directly), to verify http.ResponseController can reach
// it through the teeResponseWriter.
type flushTrackingWriter struct {
	http.ResponseWriter
	flushed bool
}

func (w *flushTrackingWriter) Flush() {
	w.flushed = true
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func TestTeeResponseWriter_UnwrapReachesFlusher(t *testing.T) {
	t.Parallel()

	underlying := &flushTrackingWriter{ResponseWriter: httptest.NewRecorder()}
	tee := newTeeResponseWriter(underlying, DefaultMaxResponseBodyBytes)

	// http.ResponseController relies on Unwrap to find the Flusher.
	if err := http.NewResponseController(tee).Flush(); err != nil {
		t.Fatalf("ResponseController.Flush via Unwrap failed: %v", err)
	}

	if !underlying.flushed {
		t.Error("expected underlying writer's Flush to be called through Unwrap")
	}
}

func TestTeeResponseWriter_OverflowStopsBuffering(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	tee := newTeeResponseWriter(rec, 4) // 4-byte cap

	// First write fits.
	if _, err := tee.Write([]byte("ab")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second write tips it over the cap.
	n, err := tee.Write([]byte("cde"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if n != 3 {
		t.Errorf("expected Write to report 3 bytes written, got %d", n)
	}

	if !tee.overflowed {
		t.Error("expected tee to be flagged overflowed")
	}

	if tee.body.Len() != 0 {
		t.Errorf("expected buffered body to be dropped, got %d bytes", tee.body.Len())
	}

	// The client still receives every byte.
	if got := rec.Body.String(); got != "abcde" {
		t.Errorf("expected client to receive full body, got %q", got)
	}
}

func TestMiddleware_OversizedResponseIsNotReplayed(t *testing.T) {
	t.Parallel()

	store := NewInMemStore()
	defer store.Close()

	var calls atomic.Int32

	mw := NewMiddleware(store, WithMaxResponseBodyBytes(8))
	handler := mw(http.HandlerFunc(func(respW http.ResponseWriter, _ *http.Request) {
		calls.Add(1)

		_, _ = respW.Write([]byte("this body is well beyond eight bytes"))
	}))

	server := httptest.NewServer(handler)
	defer server.Close()

	ctx := context.Background()

	for range 2 {
		code, _, err := sendReq(ctx, http.MethodPost, "/pay", server, "same-key", "body")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}

		if code != http.StatusOK {
			t.Fatalf("expected 200, got %d", code)
		}
	}

	// Because the oversized response was never stored, the handler runs both
	// times rather than the second being replayed.
	if got := calls.Load(); got != 2 {
		t.Errorf("expected handler to run twice (no replay), ran %d times", got)
	}
}

// gatedStore wraps a Store and lets a test pause a specific TryLock call
// between the stored-response check and the lock attempt.
type gatedStore struct {
	Store

	tryLockCalls atomic.Int32
	// gateOn is the 1-based TryLock call that must wait on gate.
	gateOn int32
	// reached is closed once the gated call has arrived at the gate.
	reached chan struct{}
	// gate blocks the gated call until closed.
	gate chan struct{}
}

func (g *gatedStore) TryLock(
	ctx context.Context,
	key string,
) (context.Context, context.CancelFunc, error) {
	if g.tryLockCalls.Add(1) == g.gateOn {
		close(g.reached)
		<-g.gate
	}

	return g.Store.TryLock(ctx, key)
}

// TestMiddleware_DuplicateAfterOriginalFinishes_ReplaysStoredResponse
// reproduces issue #43: a duplicate whose stored-response check misses
// while the original is still running, but which only reaches TryLock after
// the original has stored its response and released the lock, must replay
// the stored response instead of running the handler a second time.
func TestMiddleware_DuplicateAfterOriginalFinishes_ReplaysStoredResponse(t *testing.T) {
	t.Parallel()

	inmem := NewInMemStore()
	t.Cleanup(inmem.Close)

	store := &gatedStore{
		Store:   inmem,
		gateOn:  2, // the duplicate's TryLock
		reached: make(chan struct{}),
		gate:    make(chan struct{}),
	}

	var (
		handlerCalls atomic.Int32
		started      = make(chan struct{}, 1)
		release      = make(chan struct{})
	)

	handler := NewMiddleware(store, WithErrorToHTTPFn(errorToString))(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			handlerCalls.Add(1)

			select {
			case started <- struct{}{}:
			default:
			}

			<-release
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("created"))
		}))

	server := httptest.NewServer(handler)
	defer server.Close()

	const key = "dup-after-finish"

	type result struct {
		status int
		body   string
		err    error
	}

	original := make(chan result, 1)

	go func() {
		s, b, err := sendReq(context.Background(), http.MethodPost, "/", server, key, "body")
		original <- result{status: s, body: b, err: err}
	}()

	// The original is inside the handler, holding the lock, nothing stored yet.
	<-started

	duplicate := make(chan result, 1)

	go func() {
		s, b, err := sendReq(context.Background(), http.MethodPost, "/", server, key, "body")
		duplicate <- result{status: s, body: b, err: err}
	}()

	// The duplicate has missed the stored-response check and is paused
	// just before TryLock.
	<-store.reached

	// Let the original finish: it stores its response and releases the lock.
	close(release)

	r := <-original
	if r.err != nil {
		t.Fatalf("original request: %v", r.err)
	}

	if r.status != http.StatusCreated {
		t.Fatalf("original status = %d, want %d", r.status, http.StatusCreated)
	}

	// Now let the duplicate take the (free) lock.
	close(store.gate)

	d := <-duplicate
	if d.err != nil {
		t.Fatalf("duplicate request: %v", d.err)
	}

	if d.status != http.StatusCreated || d.body != "created" {
		t.Errorf("duplicate response = %d %q, want %d %q",
			d.status, d.body, http.StatusCreated, "created")
	}

	if got := handlerCalls.Load(); got != 1 {
		t.Errorf("handler ran %d times for one idempotency key, want 1", got)
	}
}

// failingLockStore is a Store whose TryLock always fails with lockErr.
type failingLockStore struct {
	Store

	lockErr error
}

func (f *failingLockStore) TryLock(
	_ context.Context,
	_ string,
) (context.Context, context.CancelFunc, error) {
	return nil, nil, f.lockErr
}

// TestMiddleware_TryLockErrors reproduces issue #44: only a "lock held"
// failure is a RequestInFlightError (409). Any other TryLock failure, such
// as the backing store being unreachable, must surface as a store failure
// (500) that keeps the original error.
func TestMiddleware_TryLockErrors(t *testing.T) {
	t.Parallel()

	errUnreachable := errors.New("valkey: connection refused")

	tests := []struct {
		name       string
		lockErr    error
		wantStatus int
		wantType   string
		wantWraps  error
	}{
		{
			name:       "lock held is 409",
			lockErr:    fmt.Errorf("TryLock: %w", ErrLockHeld),
			wantStatus: http.StatusConflict,
			wantType:   "RequestInFlightError",
			wantWraps:  ErrLockHeld,
		},
		{
			name:       "store failure is 500 and keeps the cause",
			lockErr:    fmt.Errorf("TryLock: %w", errUnreachable),
			wantStatus: http.StatusInternalServerError,
			wantType:   "TryLockError",
			wantWraps:  errUnreachable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inmem := NewInMemStore()
			t.Cleanup(inmem.Close)

			var (
				gotErr       = make(chan error, 1)
				handlerCalls atomic.Int32
			)

			captureErr := func(w http.ResponseWriter, r *http.Request, err error) {
				gotErr <- err

				errorToString(w, r, err)
			}

			handler := NewMiddleware(
				&failingLockStore{Store: inmem, lockErr: tt.lockErr},
				WithErrorToHTTPFn(captureErr),
			)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				handlerCalls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))

			server := httptest.NewServer(handler)
			defer server.Close()

			status, body, err := sendReq(
				context.Background(), http.MethodPost, "/", server, "lock-err-key", "body",
			)
			if err != nil {
				t.Fatalf("request: %v", err)
			}

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}

			if !strings.Contains(body, tt.wantType) {
				t.Errorf("body = %q, want it to contain %s", body, tt.wantType)
			}

			select {
			case captured := <-gotErr:
				if !errors.Is(captured, tt.wantWraps) {
					t.Errorf("error %v does not wrap %v", captured, tt.wantWraps)
				}
			default:
				t.Error("error handler was not called")
			}

			if handlerCalls.Load() != 0 {
				t.Errorf("handler ran %d times, want 0", handlerCalls.Load())
			}
		})
	}
}

// TestMiddleware_TryLockCancelledRequestIsNotReported checks that a TryLock
// failure caused by the request's own context being cancelled (client
// disconnect) is neither reported to the error handler nor treated as a
// store failure.
func TestMiddleware_TryLockCancelledRequestIsNotReported(t *testing.T) {
	t.Parallel()

	inmem := NewInMemStore()
	t.Cleanup(inmem.Close)

	var (
		errFnCalls   atomic.Int32
		handlerCalls atomic.Int32
	)

	handler := NewMiddleware(
		&failingLockStore{Store: inmem, lockErr: context.Canceled},
		WithErrorToHTTPFn(func(w http.ResponseWriter, r *http.Request, err error) {
			errFnCalls.Add(1)
			errorToString(w, r, err)
		}),
	)(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		handlerCalls.Add(1)
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader("body"))
	req.Header.Set(DefaultIdempotencyKeyHeader, "cancelled-key")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if errFnCalls.Load() != 0 {
		t.Errorf("error handler called %d times for a cancelled request, want 0", errFnCalls.Load())
	}

	if handlerCalls.Load() != 0 {
		t.Errorf("handler ran %d times, want 0", handlerCalls.Load())
	}

	if rec.Body.Len() != 0 {
		t.Errorf("body written for a cancelled request: %q", rec.Body.String())
	}
}
