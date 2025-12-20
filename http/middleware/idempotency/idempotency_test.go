package idempotency

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func BenchmarkMiddleware(b *testing.B) {
	b.ReportAllocs()

	idempotencyMiddleware := NewMiddleware(NewInMemStore())

	mux := http.NewServeMux()
	mux.Handle("/",
		idempotencyMiddleware(
			http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}),
		),
	)

	for b.Loop() {
		b.StopTimer()

		reqRec := httptest.NewRecorder()

		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/", nil)
		req.Header.Add(DefaultIdempotencyKeyHeader, strconv.Itoa(int(time.Now().Unix())))

		b.StartTimer()

		// with a new req every millisecond in mem
		mux.ServeHTTP(reqRec, req)
	}
}
