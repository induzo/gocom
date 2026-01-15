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

	idempotencyMiddleware := NewMiddleware(NewInMemStore())

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
					body:   "internal server error",
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

				go func(id int, key, body string) {
					defer wg.Done()

					time.Sleep(reqw.startAt)

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
				}(reqIdx, reqw.key, reqw.body)
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
	}, respRec, storedResp)

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

	tee := newTeeResponseWriter(httptest.NewRecorder())

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

	tee := newTeeResponseWriter(buf)

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
