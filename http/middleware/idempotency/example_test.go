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

	store := idempotency.NewInMemStore()
	defer store.Close()

	idempotencyMiddleware := idempotency.NewMiddleware(store)
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
		wg.Go(func() {
			sendPOSTReq(ctx, server, "same-key", "")
		})

		time.Sleep(80 * time.Millisecond)
	}

	// Output:
	// 400
	// {
	//   "type": "errors/missing-idempotency-key-header",
	//   "title": "missing idempotency key header",
	//   "detail": "missing idempotency key header `X-Idempotency-Key`",
	//   "instance": "/"
	// }
	// 409
	// {
	//   "type": "errors/request-already-in-flight",
	//   "title": "request already in flight",
	//   "detail": "request with key `X-Idempotency-Key`:`same-key` still in flight",
	//   "instance": "/"
	// }
	// 200
	// Hello World! 1
	// 200
	// Hello World! 1
}

func sendPOSTReq(ctx context.Context, server *httptest.Server, key, reqBody string) {
	req, _ := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		server.URL,
		bytes.NewBufferString(reqBody),
	)
	req.Header.Set(idempotency.DefaultIdempotencyKeyHeader, key)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)

		return
	}
	defer resp.Body.Close()

	fmt.Println(resp.StatusCode)

	body, errB := io.ReadAll(resp.Body)
	if errB != nil {
		fmt.Println(errB)

		return
	}

	fmt.Println(string(body))
}
