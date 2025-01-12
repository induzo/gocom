package idempotency

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
		options           []func(*config)
		expectedResp      map[int]resp
		expectedCounter   int
	}{
		{
			name:              "1 request",
			reqProcessingTime: 0,
			reqws: []req{
				{
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
					startAt: 0,
					body:    "hola",
				},
			},
			options: []func(*config){WithOptionalIdempotencyKey()},
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
					key:     "samekey",
					startAt: 0,
					body:    "hola",
				},
				{
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
					key:     "samekey",
					startAt: 0,
					body:    "hola",
				},
				{
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
					key:     "firstkey",
					startAt: 0,
					body:    "hola",
				},
				{
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
	}

	for _, tt := range testc {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			options := append([]func(*config){WithErrorToHTTPFn(errorToString)}, tt.options...)

			idempotencyMiddleware := NewMiddleware(NewInMemStore(), options...)

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

					status, body, err := sendPOSTReq(context.Background(), server, key, body)
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

func sendPOSTReq(ctx context.Context, server *httptest.Server, key, reqBody string) (int, string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, bytes.NewBufferString(reqBody))
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

// errorToString write the error type returned
func errorToString(
	_ *slog.Logger,
	writer http.ResponseWriter,
	_ *http.Request,
	_ string,
	err error,
) {
	switch {
	case errors.As(err, &MissingIdempotencyKeyHeaderError{}):
		http.Error(writer, "MissingIdempotencyKeyHeaderError", http.StatusBadRequest)
	case errors.As(err, &RequestInFlightError{}):
		fmt.Println("inflight")
		http.Error(writer, "RequestInFlightError", http.StatusConflict)
	case errors.As(err, &MismatchedSignatureError{}):
		http.Error(writer, "MismatchedSignatureError", http.StatusBadRequest)
	default:
		http.Error(
			writer,
			"internal server error",
			http.StatusInternalServerError,
		)
	}
}
