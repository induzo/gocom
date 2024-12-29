package idempotency

import (
	"net/http"
)

type Store interface {
	Set(key string, value any)
	Get(key string) (any, bool)
}

// Middleware is a middleware that adds a writable context to the request.
func Middleware(_ Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		// check if there s an idempotency key in the request
		// if there is, check if the key is in the store
		// if it is, return the value
		// if it is not, add the key to the store to "lock" the request

		next.ServeHTTP(writer, req)

		// store the response in the store
		// return the response
	})
}
