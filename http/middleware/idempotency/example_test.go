package idempotency_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/induzo/gocom/http/middleware/idempotency"
)

// Using NewMiddleware
func ExampleNewMiddleware() {
	ctx := context.Background()
	idempotencyMiddleware := idempotency.NewMiddleware(idempotency.NewInMemStore())
	mux := http.NewServeMux()

	counter := int32(0)

	mux.Handle("/",
		idempotencyMiddleware(
			http.HandlerFunc(func(respW http.ResponseWriter, _ *http.Request) {
				time.Sleep(100 * time.Millisecond)

				atomic.AddInt32(&counter, 1)

				respW.Write([]byte("Hello World! " + strconv.Itoa(int(counter))))
			})),
	)

	// Serve the handler with http test server
	server := httptest.NewServer(mux)
	defer server.Close()

	// send a first req without a key
	sendPOSTReq(ctx, server, "", "")

	var wg sync.WaitGroup

	for range 3 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			sendPOSTReq(ctx, server, "same-key", "")
		}()

		time.Sleep(80 * time.Millisecond)
	}

	// add a request that does not have the same signature
	wg.Add(1)

	go func() {
		defer wg.Done()

		sendPOSTReq(ctx, server, "same-key", "diff-body")
	}()

	wg.Wait()

	// add a request that does not have the same id key
	sendPOSTReq(ctx, server, "diff-key", "")

	// Output:
	// 400
	// {
	//   "type": "https://example.com/errors/missing-idempotency-key-header",
	//   "title": "missing idempotency key header",
	//   "detail": "missing idempotency key header `X-Idempotency-Key` for request to POST /",
	//   "instance": "/"
	// }
	// 409
	// {
	//   "type": "https://example.com/errors/request-already-in-flight",
	//   "title": "request already in flight",
	//   "detail": "request to `POST /` `same-key` still in flight",
	//   "instance": "/"
	// }
	// 200
	// Hello World! 1
	// 200
	// Hello World! 1
	// 400
	// {
	//   "type": "https://example.com/errors/mismatched-signature",
	//   "title": "mismatched signature",
	//   "detail": "mismatched signature for request to `POST /` `same-key`",
	//   "instance": "/"
	// }
	// 200
	// Hello World! 2
}

func sendPOSTReq(ctx context.Context, server *httptest.Server, key, reqBody string) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, bytes.NewBufferString(reqBody))
	req.Header.Set("X-Idempotency-Key", key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)

	body, errB := io.ReadAll(resp.Body)
	if errB != nil {
		fmt.Println(errB)
	}

	fmt.Println(string(body))
}
