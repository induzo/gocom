package health

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/goleak"
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
			err:            &CheckError{name: "test", err: errors.New("err")},
			expectedString: fmt.Sprintf("health check function: %s returned err: %v", "test", errors.New("err")),
		},
	}

	for _, tt := range tests {
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
		checkErr   = &CheckError{name: "check", err: errors.New("failed to ping db")}
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
						CheckFn: func(_ context.Context) error {
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
						CheckFn: func(_ context.Context) error {
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
						CheckFn: func(_ context.Context) error {
							return errors.New("failed to ping db")
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
						CheckFn: func(_ context.Context) error {
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
						CheckFn: func(_ context.Context) error {
							return nil
						},
					},
					{
						Name:    "timeout",
						Timeout: time.Millisecond,
						CheckFn: func(_ context.Context) error {
							time.Sleep(20 * time.Millisecond)

							return nil
						},
					},
					{
						Name: "check",
						CheckFn: func(_ context.Context) error {
							return errors.New("failed to ping db")
						},
					},
					{
						Name:    "no timeout",
						Timeout: 50 * time.Millisecond,
						CheckFn: func(_ context.Context) error {
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

func BenchmarkHealth(b *testing.B) {
	rr := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodGet, HealthEndpoint, nil)

	health := NewHealth(WithChecks(CheckConfig{
		Name:    "test",
		Timeout: 5 * time.Second,
		CheckFn: func(_ context.Context) error {
			return nil
		},
	}))

	handler := health.Handler()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		handler.ServeHTTP(rr, req)
	}
}
