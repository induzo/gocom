package idempotency_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/induzo/gocom/http/middleware/idempotency"
)

// Using NewMiddleware
func ExampleNewMiddleware() {
	idempotencyMiddleware := idempotency.NewMiddleware(idempotency.NewInMemStore())

	mux := http.NewServeMux()

	mux.Handle("/",
		idempotencyMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Hello World!"))
			})),
	)

	rr := httptest.NewRecorder()

	// Serve the handler
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	fmt.Println(rr.Body.String())

	// Output:
	// Hello World!
}
