package writablecontext

import (
	"net/http"
	"net/http/httptest"
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
				if store == nil {
					return
				}

				val, ok := store.Get(tt.keyGet)
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

			if tt.withMiddleware && tt.withWrittenValue {
				mux.Handle("/", Middleware(middlewareSetInStore(http.HandlerFunc(handler))))
			}

			if tt.withMiddleware && !tt.withWrittenValue {
				mux.Handle("/", Middleware(http.HandlerFunc(handler)))
			}

			if !tt.withMiddleware && tt.withWrittenValue {
				mux.Handle("/", middlewareSetInStore(http.HandlerFunc(handler)))
			}

			if !tt.withMiddleware && !tt.withWrittenValue {
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
	mux.Handle("/", Middleware(middlewareSetInStore(http.HandlerFunc(handler))))

	rr := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	b.ResetTimer()

	for range b.N {
		mux.ServeHTTP(rr, req)
	}
}

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(
		m,
		goleak.IgnoreAnyFunction(
			"github.com/testcontainers/testcontainers-go.(*Reaper).Connect.func1",
		),
	)
}
