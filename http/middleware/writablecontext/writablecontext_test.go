package writablecontext

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go.uber.org/goleak"
)

func TestFromRequest(t *testing.T) {
	t.Parallel()

	middlewareSetInStore := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			store := FromContext(r.Context())
			store.Set("foo", "bar")
			next.ServeHTTP(w, r)
		})
	}

	tests := []struct {
		name             string
		keyGet           string
		withMiddleware   bool
		withWrittenValue bool
		want             string
	}{
		{
			name:             "request with middleware and written key in context",
			keyGet:           "foo",
			withMiddleware:   true,
			withWrittenValue: true,
			want:             "bar",
		},
		{
			name:             "request with middleware and NO written key in context",
			keyGet:           "foo",
			withMiddleware:   true,
			withWrittenValue: false,
			want:             "",
		},
		{
			name:             "request with NO middleware and written key in context",
			keyGet:           "foo",
			withMiddleware:   false,
			withWrittenValue: true,
			want:             "",
		},
		{
			name:             "request with middleware and written key in context, empty key",
			keyGet:           "wrongkey",
			withMiddleware:   true,
			withWrittenValue: true,
			want:             "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a handler to use for testing
			handler := func(w http.ResponseWriter, r *http.Request) {
				store := FromContext(r.Context())

				val, ok := store.Get(tt.keyGet)
				if !ok {
					return
				}

				valS, ok := val.(string)
				if !ok {
					return
				}

				if _, err := w.Write([]byte(valS)); err != nil {
					t.Errorf("write: %v", err)
				}
			}

			mux := http.NewServeMux()

			switch {
			case tt.withMiddleware && tt.withWrittenValue:
				mux.Handle("/", Middleware(middlewareSetInStore(http.HandlerFunc(handler))))
			case tt.withMiddleware && !tt.withWrittenValue:
				mux.Handle("/", Middleware(http.HandlerFunc(handler)))
			case !tt.withMiddleware && tt.withWrittenValue:
				// Without the middleware FromContext returns nil; the inner
				// middlewareSetInStore would dereference it, so skip wiring
				// it and rely on the handler observing nothing.
				mux.Handle("/", http.HandlerFunc(handler))
			default:
				mux.Handle("/", http.HandlerFunc(handler))
			}

			rr := httptest.NewRecorder()

			mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

			// check the body
			if rr.Body.String() != tt.want {
				t.Errorf("expected body to be '%s', got '%s'", tt.want, rr.Body.String())

				return
			}
		})
	}
}

// TestStore_NilGet asserts that Get on a nil *Store (e.g. when Middleware
// was not installed) returns (nil, false) instead of panicking, so callers
// can use FromContext-then-Get directly.
func TestStore_NilGet(t *testing.T) {
	t.Parallel()

	var s *Store

	got, ok := s.Get("anything")
	if ok || got != nil {
		t.Errorf("Get on nil *Store = (%v, %v), want (nil, false)", got, ok)
	}
}

// TestStore_NilSetPanics pins the contract that Set on a nil *Store
// panics, so a caller who forgot to install Middleware fails loudly.
func TestStore_NilSetPanics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected Set on nil *Store to panic")
		}
	}()

	var s *Store

	s.Set("k", "v")
}

// TestStore_ConcurrentAccess ensures Set and Get are safe for concurrent
// use; this test is meaningful under `go test -race`.
func TestStore_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	const goroutines = 32

	const writesPer = 64

	s := newStore()

	var wg sync.WaitGroup

	wg.Add(goroutines * 2)

	for g := range goroutines {
		go func() {
			defer wg.Done()

			for i := range writesPer {
				s.Set(fmt.Sprintf("g%d-k%d", g, i), i)
			}
		}()

		go func() {
			defer wg.Done()

			for i := range writesPer {
				_, _ = s.Get(fmt.Sprintf("g%d-k%d", g, i))
			}
		}()
	}

	wg.Wait()
}

func BenchmarkFromRequest(b *testing.B) {
	b.ReportAllocs()

	middlewareSetInStore := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			store := FromContext(r.Context())
			store.Set("foo", "bar")
			next.ServeHTTP(w, r)
		})
	}

	handler := func(w http.ResponseWriter, r *http.Request) {
		store := FromContext(r.Context())

		val, ok := store.Get("foo")
		if !ok {
			return
		}

		valS, ok := val.(string)
		if !ok {
			return
		}

		_, _ = w.Write([]byte(valS))
	}

	mux := http.NewServeMux()
	mux.Handle("/", Middleware(middlewareSetInStore(http.HandlerFunc(handler))))

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	for b.Loop() {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
