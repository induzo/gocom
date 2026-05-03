package health

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goccy/go-json"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestTimeoutError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		err            error
		expectedString string
	}{
		{
			name: "happy path",
			err:  &TimeoutError{name: "test", timeElapsed: time.Second},
			expectedString: fmt.Sprintf(
				"health check function: %s timed out after %v",
				"test",
				time.Second,
			),
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
			name: "happy path",
			err:  &CheckError{name: "test", err: errors.New("err")},
			expectedString: fmt.Sprintf(
				"health check function: %s returned err: %v",
				"test",
				errors.New("err"),
			),
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

	h := NewHealth()
	if h == nil {
		t.Fatal("NewHealth returned nil")
	}

	if len(h.checks) != 0 {
		t.Errorf("expected zero registered checks, got %d", len(h.checks))
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
				t.Errorf("expected status %d, got %d", tt.wantStatusCode, resp.StatusCode)
			}

			if tt.wantErrResponse == nil {
				body, _ := io.ReadAll(rr.Body)

				trimmedBody := strings.TrimSpace(string(body))
				if trimmedBody != "" {
					t.Errorf("expected empty response")
				}

				return
			}

			if got := resp.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}

			if got := resp.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
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

// TestHealth_PanicRecovery ensures a panicking CheckFn does not crash the
// server: the handler must respond 503 with a check_error code.
func TestHealth_PanicRecovery(t *testing.T) {
	t.Parallel()

	h := NewHealth(WithChecks(CheckConfig{
		Name: "panicker",
		CheckFn: func(_ context.Context) error {
			panic("boom")
		},
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, HealthEndpoint, nil)

	h.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}

	var got Response
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if got.Error != "check_error" {
		t.Errorf("Error code = %q, want check_error", got.Error)
	}

	if !strings.Contains(got.ErrorMessage, "panic") {
		t.Errorf("ErrorMessage = %q, want it to mention panic", got.ErrorMessage)
	}
}

// TestHealth_NonGET asserts the handler currently accepts non-GET methods
// and runs the configured checks. Pinning this behavior so any future
// method-enforcement change is intentional.
func TestHealth_NonGET(t *testing.T) {
	t.Parallel()

	h := NewHealth(WithChecks(CheckConfig{
		Name: "ok",
		CheckFn: func(_ context.Context) error {
			return nil
		},
	}))

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(method, HealthEndpoint, nil)

		h.Handler().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("method %s: status = %d, want %d", method, rr.Code, http.StatusOK)
		}
	}
}

// TestHealth_ContextCanceled verifies the handler still returns 503 when the
// request context is cancelled before the check completes.
func TestHealth_ContextCanceled(t *testing.T) {
	t.Parallel()

	released := make(chan struct{})

	h := NewHealth(WithChecks(CheckConfig{
		Name:    "slow",
		Timeout: time.Second,
		CheckFn: func(ctx context.Context) error {
			defer close(released)

			<-ctx.Done()

			return ctx.Err()
		},
	}))

	ctx, cancel := context.WithCancel(context.Background())

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, HealthEndpoint, nil)

	cancel() // cancel before serving so the handler immediately observes Done

	h.Handler().ServeHTTP(rr, req)
	<-released

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

// TestHealth_NoSuperfluousWriteHeader catches the historical
// "http: superfluous response.WriteHeader call" warning from the failure
// path by counting WriteHeader invocations on a tracking ResponseWriter.
func TestHealth_NoSuperfluousWriteHeader(t *testing.T) {
	t.Parallel()

	h := NewHealth(WithChecks(CheckConfig{
		Name: "broken",
		CheckFn: func(_ context.Context) error {
			return errors.New("boom")
		},
	}))

	rr := &countingResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	req := httptest.NewRequest(http.MethodGet, HealthEndpoint, nil)

	h.Handler().ServeHTTP(rr, req)

	if rr.writeHeaderCalls.Load() != 1 {
		t.Errorf("WriteHeader called %d times, want 1", rr.writeHeaderCalls.Load())
	}
}

type countingResponseWriter struct {
	*httptest.ResponseRecorder
	writeHeaderCalls atomic.Int32
}

func (w *countingResponseWriter) WriteHeader(status int) {
	w.writeHeaderCalls.Add(1)
	w.ResponseRecorder.WriteHeader(status)
}

func BenchmarkHealth(b *testing.B) {
	health := NewHealth(WithChecks(CheckConfig{
		Name:    "test",
		Timeout: 5 * time.Second,
		CheckFn: func(_ context.Context) error {
			return nil
		},
	}))

	handler := health.Handler()

	for b.Loop() {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, HealthEndpoint, nil)
		handler.ServeHTTP(rr, req)
	}
}
