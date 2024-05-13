package redisinit_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/redis/go-redis/v9"

	"github.com/induzo/gocom/database/redisinit"
)

// Using standard net/http package. We can also simply pass healthCheck as a CheckFn in gocom/transport/http/health/v2.
//
//nolint:testableexamples // cannot run without redis
func ExampleClientHealthCheck() {
	ctx := context.Background()

	cli := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	healthCheck := redisinit.ClientHealthCheck(cli)

	mux := http.NewServeMux()

	mux.HandleFunc("/sys/health", func(rw http.ResponseWriter, _ *http.Request) {
		if err := healthCheck(ctx); err != nil {
			rw.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/sys/health", nil)
	nr := httptest.NewRecorder()

	mux.ServeHTTP(nr, req)

	rr := nr.Result()
	defer rr.Body.Close()

	fmt.Println(rr.StatusCode)
}
