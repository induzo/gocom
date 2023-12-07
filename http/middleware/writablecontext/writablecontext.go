package writablecontext

import (
	"context"
	"net/http"
)

// Key is the type for the context key.
type Key string

// contextKey is the key for the context.
const contextKey Key = "writeablecontext"

// Store is a map of key/value pairs for the context.
type Store map[string]any

func newStore() Store {
	return make(Store)
}

// Set sets the value for a key in the context.
func (s Store) Set(key string, value any) {
	if s == nil {
		s = newStore()
	}

	s[key] = value
}

// Get returns the value for a key in the context.
func (s Store) Get(key string) (any, bool) {
	val, ok := s[key]

	return val, ok
}

// Middleware is a middleware that adds a writable context to the request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newR := r.WithContext(context.WithValue(r.Context(), contextKey, newStore()))
		next.ServeHTTP(w, newR)
	})
}

// FromContext returns the writable context from the request.
func FromContext(ctx context.Context) Store {
	currStore, ok := ctx.Value(contextKey).(Store)
	if !ok {
		return nil
	}

	return currStore
}
