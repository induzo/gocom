// Package writablecontext provides an HTTP middleware that injects a
// request-scoped, mutable bag (Store) into the request context. Downstream
// handlers can attach values that upstream middleware (typically access
// loggers or tracing) reads after the inner handler returns.
//
// The Store is safe for concurrent use by goroutines spawned during a
// single request.
package writablecontext

import (
	"context"
	"net/http"
	"sync"
)

// ctxKey is unexported so callers cannot construct colliding keys.
type ctxKey struct{}

// Store is a request-scoped, mutable bag of key/value pairs. A Store is
// created by Middleware on every request and lives for the duration of
// that request. Get and Set are safe for concurrent use.
type Store struct {
	mu   sync.RWMutex
	data map[string]any
}

func newStore() *Store {
	return &Store{data: make(map[string]any)}
}

// Set stores value under key. Set panics if called on a nil *Store; obtain
// a Store via FromContext after installing Middleware.
func (s *Store) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value
}

// Get returns the value stored under key, if any. Calling Get on a nil
// *Store returns the zero value and false, so callers can use the result
// of FromContext directly.
func (s *Store) Get(key string) (any, bool) {
	if s == nil {
		return nil, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]

	return val, ok
}

// Middleware attaches a fresh writable Store to the request context for
// the lifetime of each request.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newR := r.WithContext(context.WithValue(r.Context(), ctxKey{}, newStore()))
		next.ServeHTTP(w, newR)
	})
}

// FromContext returns the Store added to ctx by Middleware. It returns nil
// if Middleware is not installed in the chain; callers may invoke Get on
// the returned value safely either way.
func FromContext(ctx context.Context) *Store {
	s, _ := ctx.Value(ctxKey{}).(*Store)

	return s
}
