package writablecontext_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/induzo/gocom/http/middleware/writablecontext"
)

// Using Middleware
func ExampleMiddleware() {
	middlewareSetInStore := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			store := writablecontext.FromContext(r.Context())
			store.Set("foo", "bar")
			next.ServeHTTP(w, r)
		})
	}

	// Create a handler to use for testing
	handler := func(w http.ResponseWriter, r *http.Request) {
		store := writablecontext.FromContext(r.Context())
		if store == nil {
			return
		}

		val, ok := store.Get("foo")
		if !ok {
			return
		}

		valS, ok := val.(string)
		if !ok {
			return
		}

		w.Write([]byte(valS))
	}

	mux := http.NewServeMux()

	mux.Handle("/", writablecontext.Middleware(middlewareSetInStore(http.HandlerFunc(handler))))

	rr := httptest.NewRecorder()

	// Serve the handler
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	fmt.Println(rr.Body.String())

	// Output:
	// bar
}
