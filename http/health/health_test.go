package health

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/goleak"

	"github.com/induzo/gocom/database/pginit"
)

func TestMain(m *testing.M) {
	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	if *leak {
		goleak.VerifyTestMain(m)

		return
	}

	os.Exit(m.Run())
}

func TestTimeoutError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		expectedString string
	}{
		{
			name:           "happy path",
			err:            &TimeoutError{name: "test", timeElapsed: time.Second},
			expectedString: fmt.Sprintf("health check function: %s timed out after %v", "test", time.Second),
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err.Error() != tt.expectedString {
				t.Error("unexpected Error string")
			}
		})
	}
}

func TestCheckError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		expectedString string
	}{
		{
			name:           "happy path",
			err:            &CheckError{name: "test", err: fmt.Errorf("err")},
			expectedString: fmt.Sprintf("health check function: %s returned err: %v", "test", fmt.Errorf("err")),
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.err.Error() != tt.expectedString {
				t.Error("unexpected Error string")
			}
		})
	}
}

func TestNewHealth(t *testing.T) {
	t.Parallel()

	if health := NewHealth(); reflect.TypeOf(health) != reflect.TypeOf(&Health{}) {
		t.Error("returned struct is not of type Health")
	}
}

func TestHealth(t *testing.T) {
	t.Parallel()

	var (
		checkErr   = &CheckError{name: "check", err: fmt.Errorf("failed to ping db")}
		timeoutErr = &TimeoutError{name: "timeout", timeElapsed: time.Millisecond}
	)

	type args struct {
		checkConfigs []CheckConfig
	}

	tests := []struct {
		name            string
		args            args
		wantStatusCode  int
		wantErrResponse []Response
	}{
		{
			name: "happy path",
			args: args{
				checkConfigs: []CheckConfig{
					{
						Name:    "happy",
						Timeout: 50 * time.Millisecond,
						CheckFn: func(ctx context.Context) error {
							return nil
						},
					},
				},
			},
			wantStatusCode:  http.StatusOK,
			wantErrResponse: nil,
		},
		{
			name: "default timeout",
			args: args{
				checkConfigs: []CheckConfig{
					{
						Name: "default timeout",
						CheckFn: func(ctx context.Context) error {
							return nil
						},
					},
				},
			},
			wantStatusCode:  http.StatusOK,
			wantErrResponse: nil,
		},
		{
			name: "health check error",
			args: args{
				checkConfigs: []CheckConfig{
					{
						Name: "check",
						CheckFn: func(ctx context.Context) error {
							return fmt.Errorf("failed to ping db")
						},
					},
				},
			},
			wantStatusCode:  http.StatusServiceUnavailable,
			wantErrResponse: []Response{{ErrorMessage: checkErr.Error(), Error: "check_error"}},
		},
		{
			name: "timeout error",
			args: args{
				checkConfigs: []CheckConfig{
					{
						Name:    "timeout",
						Timeout: time.Millisecond,
						CheckFn: func(ctx context.Context) error {
							time.Sleep(20 * time.Millisecond)

							return nil
						},
					},
				},
			},
			wantStatusCode:  http.StatusServiceUnavailable,
			wantErrResponse: []Response{{ErrorMessage: timeoutErr.Error(), Error: "timeout_error"}},
		},
		{
			name: "multiple checks",
			args: args{
				checkConfigs: []CheckConfig{
					{
						Name: "happy",
						CheckFn: func(ctx context.Context) error {
							return nil
						},
					},
					{
						Name:    "timeout",
						Timeout: time.Millisecond,
						CheckFn: func(ctx context.Context) error {
							time.Sleep(20 * time.Millisecond)

							return nil
						},
					},
					{
						Name: "check",
						CheckFn: func(ctx context.Context) error {
							return fmt.Errorf("failed to ping db")
						},
					},
					{
						Name:    "no timeout",
						Timeout: 50 * time.Millisecond,
						CheckFn: func(ctx context.Context) error {
							time.Sleep(10 * time.Millisecond)

							return nil
						},
					},
				},
			},
			wantStatusCode: http.StatusServiceUnavailable,
			wantErrResponse: []Response{
				{ErrorMessage: timeoutErr.Error(), Error: "timeout_error"},
				{ErrorMessage: checkErr.Error(), Error: "check_error"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, HealthEndpoint, nil)

			health := NewHealth(WithChecks(tt.args.checkConfigs...))
			handler := health.Handler()
			handler.ServeHTTP(rr, req)

			resp := rr.Result()
			defer resp.Body.Close()

			if status := resp.StatusCode; status != tt.wantStatusCode {
				t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
			}

			if tt.wantErrResponse == nil {
				body, _ := io.ReadAll(rr.Body)
				trimmedBody := strings.TrimSpace(string(body))

				if trimmedBody != "" {
					t.Errorf("expected empty response")
				}

				return
			}

			gotRes := Response{}

			if err := json.NewDecoder(rr.Body).Decode(&gotRes); err != nil {
				t.Fatalf("fail to decode response body: %s", rr.Body)
			}

			found := false
			for _, wantRes := range tt.wantErrResponse {
				if reflect.DeepEqual(gotRes, wantRes) {
					found = true
				}
			}
			if !found {
				t.Errorf("expected response in %v, got response %v", gotRes, tt.wantErrResponse)
			}
		})
	}
}

func TestHealth_Redis(t *testing.T) {
	t.Parallel()

	var err error

	dockerPool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker: %s", err)
	}

	resource, err := dockerPool.Run("redis", "7.0.4", nil)
	if err != nil {
		t.Fatalf("Could not start resource: %s", err)
	}

	defer func() {
		if err = dockerPool.Purge(resource); err != nil {
			t.Fatalf("Could not purge resource: %s", err)
		}
	}()

	var redisCli *redis.Client

	if errP := dockerPool.Retry(func() error {
		redisCli = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("localhost:%s", resource.GetPort("6379/tcp")),
		})

		return redisCli.Ping(context.Background()).Err()
	}); errP != nil {
		t.Fatalf("Could not connect to docker: %s", errP)
	}

	defer redisCli.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, HealthEndpoint, nil)

	health := NewHealth(WithChecks(
		CheckConfig{
			Name:    "redis",
			Timeout: 1 * time.Second,
			CheckFn: func(ctx context.Context) error {
				return redisCli.Ping(ctx).Err()
			},
		}))

	handler := health.Handler()
	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, _ := io.ReadAll(rr.Body)
	trimmedBody := strings.TrimSpace(string(body))

	if trimmedBody != "" {
		t.Errorf("expected empty response")
	}
}

func TestHealth_Pgx(t *testing.T) {
	t.Parallel()

	dockerPool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker: %s", err)
	}

	resource, err := dockerPool.Run("postgres", "14", []string{
		"POSTGRES_PASSWORD=postgres",
		"POSTGRES_USER=postgres",
		"POSTGRES_DB=datawarehouse",
		"listen_addresses = '*'",
	})
	if err != nil {
		t.Fatalf("Could not start resource: %s", err)
	}

	defer func() {
		if err = dockerPool.Purge(resource); err != nil {
			t.Fatalf("Could not purge resource: %s", err)
		}
	}()

	var connPool *pgxpool.Pool

	if errP := dockerPool.Retry(func() error {
		pgi, errI := pginit.New(
			&pginit.Config{
				Host:         "localhost",
				Port:         strings.Split(getHostPort(resource, "5432/tcp"), ":")[1],
				User:         "postgres",
				Password:     "postgres",
				Database:     "datawarehouse",
				MaxConns:     10,
				MaxIdleConns: 10,
				MaxLifeTime:  1 * time.Minute,
			})
		if errI != nil {
			return errI
		}

		var errC error
		connPool, errC = pgi.ConnPool(context.Background())
		if errC != nil {
			return errC
		}

		return connPool.Ping(context.Background())
	}); errP != nil {
		t.Fatalf("Could not connect to docker: %s", errP)
	}

	defer connPool.Close()

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, HealthEndpoint, nil)

	health := NewHealth(WithChecks(CheckConfig{
		Name:    "pgx",
		Timeout: 1 * time.Second,
		CheckFn: func(ctx context.Context) error {
			return connPool.Ping(ctx)
		},
	}))

	handler := health.Handler()
	handler.ServeHTTP(rr, req)

	resp := rr.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, _ := io.ReadAll(rr.Body)
	trimmedBody := strings.TrimSpace(string(body))

	if trimmedBody != "" {
		t.Errorf("expected empty response")
	}
}

func getHostPort(resource *dockertest.Resource, id string) string {
	dockerURL := os.Getenv("DOCKER_HOST")
	if dockerURL == "" {
		hostAndPort := resource.GetHostPort("5432/tcp")
		hp := strings.Split(hostAndPort, ":")
		testRefHost := hp[0]
		testRefPort := hp[1]

		return testRefHost + ":" + testRefPort
	}

	u, err := url.Parse(dockerURL)
	if err != nil {
		panic(err)
	}

	return u.Hostname() + ":" + resource.GetPort(id)
}

func BenchmarkHealth(b *testing.B) {
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, HealthEndpoint, nil)

	health := NewHealth(WithChecks(CheckConfig{
		Name:    "test",
		Timeout: 5 * time.Second,
		CheckFn: func(ctx context.Context) error {
			return nil
		},
	}))

	handler := health.Handler()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rr, req)
	}
}
