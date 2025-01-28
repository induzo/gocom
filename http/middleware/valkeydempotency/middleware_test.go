package valkeydempotency

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/induzo/gocom/http/middleware/idempotency"
	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylock"
)

func TestNewMiddleware(t *testing.T) {
	t.Parallel()

	idempotencyMiddleware, closeLock, errM := NewMiddleware(
		&valkeylock.LockerOption{
			ClientOption:   valkey.ClientOption{InitAddress: []string{testValkeyPortHost}},
			KeyMajority:    1,    // Use KeyMajority=1 if you have only one Valkey instance. Also make sure that all your `Locker`s share the same KeyMajority.
			NoLoopTracking: true, // Enable this to have better performance if all your Valkey are >= 7.0.5.
		},
		1*time.Second,
	)
	if errM != nil {
		t.Fatalf("NewMiddleware: %v", errM)
	}

	defer closeLock()

	if idempotencyMiddleware == nil {
		t.Error("NewMiddleware returned nil")
	}
}

func TestMiddleware_ServeHTTP(t *testing.T) {
	t.Parallel()

	type req struct {
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
		name              string
		reqProcessingTime time.Duration
		reqws             []req
		options           []idempotency.Option
		expectedResp      map[int]resp
		expectedCounter   int
	}{
		// {
		// 	name:              "1 request",
		// 	reqProcessingTime: 0,
		// 	reqws: []req{
		// 		{
		// 			method:  http.MethodPost,
		// 			key:     "onekey",
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 	},
		// 	options: nil,
		// 	expectedResp: map[int]resp{
		// 		0: {
		// 			key:    "onekey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 	},
		// 	expectedCounter: 1,
		// },
		// {
		// 	name:              "1 request, missing idempot header",
		// 	reqProcessingTime: 0,
		// 	reqws: []req{
		// 		{
		// 			method:  http.MethodPost,
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 	},
		// 	options: nil,
		// 	expectedResp: map[int]resp{
		// 		0: {
		// 			key:    "",
		// 			status: http.StatusBadRequest,
		// 			body:   "MissingIdempotencyKeyHeaderError",
		// 		},
		// 	},
		// 	expectedCounter: 0,
		// },
		// {
		// 	name:              "1 request, missing idempot header, but optional",
		// 	reqProcessingTime: 0,
		// 	reqws: []req{
		// 		{
		// 			method:  http.MethodPost,
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 	},
		// 	options: []idempotency.Option{idempotency.WithOptionalIdempotencyKey()},
		// 	expectedResp: map[int]resp{
		// 		0: {
		// 			key:    "onekey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 	},
		// 	expectedCounter: 1,
		// },
		{
			name:              "2 concurrent requests",
			reqProcessingTime: 100 * time.Millisecond,
			reqws: []req{
				{
					method:  http.MethodPost,
					key:     "samekey",
					startAt: 0,
					body:    "hola",
				},
				{
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
		// {
		// 	name:              "2 requests, 1 after the other",
		// 	reqProcessingTime: 0,
		// 	reqws: []req{
		// 		{
		// 			method:  http.MethodPost,
		// 			key:     "samekey",
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 		{
		// 			method:  http.MethodPost,
		// 			key:     "samekey",
		// 			startAt: 20 * time.Millisecond,
		// 			body:    "hola",
		// 		},
		// 	},
		// 	options: nil,
		// 	expectedResp: map[int]resp{
		// 		0: {
		// 			key:    "samekey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 		1: {
		// 			key:    "samekey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 	},
		// 	expectedCounter: 1,
		// },
		// {
		// 	name:              "2 totally diff requests",
		// 	reqProcessingTime: 0,
		// 	reqws: []req{
		// 		{
		// 			method:  http.MethodPost,
		// 			key:     "firstkey",
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 		{
		// 			method:  http.MethodPost,
		// 			key:     "secondkey",
		// 			startAt: 20 * time.Millisecond,
		// 			body:    "hola",
		// 		},
		// 	},
		// 	options: nil,
		// 	expectedResp: map[int]resp{
		// 		0: {
		// 			key:    "firstkey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 		1: {
		// 			key:    "secondkey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 	},
		// 	expectedCounter: 2,
		// },
		// {
		// 	name:              "get request",
		// 	reqProcessingTime: 0,
		// 	reqws: []req{
		// 		{
		// 			method:  http.MethodGet,
		// 			key:     "getkey",
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 		{
		// 			method:  http.MethodGet,
		// 			key:     "getkey",
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 	},
		// 	options: nil,
		// 	expectedResp: map[int]resp{
		// 		0: {
		// 			key:    "getkey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 		1: {
		// 			key:    "getkey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 	},
		// 	expectedCounter: 2,
		// },
		// {
		// 	name:              "1 request with failing fingerprinter",
		// 	reqProcessingTime: 0,
		// 	reqws: []req{
		// 		{
		// 			method:  http.MethodPost,
		// 			key:     "onekey",
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 	},
		// 	options: []idempotency.Option{
		// 		idempotency.WithFingerprinter(
		// 			func(_ *http.Request) ([]byte, error) {
		// 				return nil, errors.New("fingerprinter error")
		// 			},
		// 		),
		// 	},
		// 	expectedResp: map[int]resp{
		// 		0: {
		// 			key:    "onekey",
		// 			status: http.StatusInternalServerError,
		// 			body:   "internal server error",
		// 		},
		// 	},
		// 	expectedCounter: 0,
		// },
		// {
		// 	name:              "1 request that exists in store but with diff fingerprint",
		// 	reqProcessingTime: 0,
		// 	reqws: []req{
		// 		{
		// 			method:  http.MethodPost,
		// 			key:     "onekey",
		// 			startAt: 0,
		// 			body:    "hola",
		// 		},
		// 		{
		// 			method:  http.MethodPost,
		// 			key:     "onekey",
		// 			startAt: 10 * time.Millisecond,
		// 			body:    "hola",
		// 		},
		// 	},
		// 	options: []idempotency.Option{
		// 		idempotency.WithFingerprinter(
		// 			func(_ *http.Request) ([]byte, error) {
		// 				return []byte(time.Now().Format(time.RFC3339Nano)), nil
		// 			},
		// 		),
		// 	},
		// 	expectedResp: map[int]resp{
		// 		0: {
		// 			key:    "onekey",
		// 			status: http.StatusOK,
		// 			body:   "hola",
		// 		},
		// 		1: {
		// 			key:    "onekey",
		// 			status: http.StatusBadRequest,
		// 			body:   "MismatchedSignatureError",
		// 		},
		// 	},
		// 	expectedCounter: 1,
		// },
	}

	for idx, tt := range testc {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			options := append([]idempotency.Option{idempotency.WithErrorToHTTPFn(errorToString)}, tt.options...)

			idempotencyMiddleware, closeLock, errM := NewMiddleware(
				&valkeylock.LockerOption{
					ClientOption:   valkey.ClientOption{InitAddress: []string{testValkeyPortHost}},
					KeyPrefix:      "valock_" + strconv.Itoa(idx) + "_",
					KeyMajority:    1,    // Use KeyMajority=1 if you have only one Valkey instance. Also make sure that all your `Locker`s share the same KeyMajority.
					NoLoopTracking: true, // Enable this to have better performance if all your Valkey are >= 7.0.5.
				},
				10*time.Second,
				options...,
			)
			if errM != nil {
				t.Fatalf("NewMiddleware: %v", errM)
			}

			defer closeLock()

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

					status, body, err := sendReq(context.Background(), reqw.method, server, key, body)
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

func sendReq(ctx context.Context, method string, server *httptest.Server, key, reqBody string) (int, string, error) {
	req, _ := http.NewRequestWithContext(ctx, method, server.URL, bytes.NewBufferString(reqBody))
	req.Header.Set(idempotency.DefaultIdempotencyKeyHeader, key)

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

// errorToString write the error type returned
func errorToString(
	writer http.ResponseWriter,
	_ *http.Request,
	err error,
) {
	switch {
	case errors.As(err, &idempotency.MissingIdempotencyKeyHeaderError{}):
		http.Error(writer, "MissingIdempotencyKeyHeaderError", http.StatusBadRequest)
	case errors.As(err, &idempotency.RequestInFlightError{}):
		fmt.Println("inflight")
		http.Error(writer, "RequestInFlightError", http.StatusConflict)
	case errors.As(err, &idempotency.MismatchedSignatureError{}):
		http.Error(writer, "MismatchedSignatureError", http.StatusBadRequest)
	case errors.As(err, &idempotency.StoreResponseError{}):
		http.Error(writer, "", http.StatusOK)
	case errors.As(err, &idempotency.GetStoredResponseError{}):
		http.Error(writer, "internal server error", http.StatusInternalServerError)
	default:
		http.Error(
			writer,
			"internal server error",
			http.StatusInternalServerError,
		)
	}
}
