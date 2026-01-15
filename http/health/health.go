// HTTP handler that retrieves health status of the application
package health

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/goccy/go-json"
	"golang.org/x/sync/errgroup"
)

const (
	HealthEndpoint = "/sys/health" // URL used by infra team
)

const (
	DefaultTimeout = 3 * time.Second
)

// TimeoutError is an error returned when the health check function exceeds the timeout duration.
type TimeoutError struct {
	name        string
	timeElapsed time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("health check function: %s timed out after %v", e.name, e.timeElapsed)
}

// CheckError is an error returned when the health check function returns an error.
type CheckError struct {
	name string
	err  error
}

func (e *CheckError) Error() string {
	return fmt.Sprintf("health check function: %s returned err: %v", e.name, e.err)
}

// CheckConfig are the parameters used to run each check.
type CheckConfig struct {
	Name    string
	CheckFn func(ctx context.Context) error
	Timeout time.Duration
}

// Response is the health check handler response.
type Response struct {
	Error        string `json:"error,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// Health provides http.Handler that retrieves the health status of the application based on the provided checks.
type Health struct {
	checks map[string]CheckConfig
}

// Option is the options type to configure Health.
type Option func(*Health)

// NewHealth returns a new Health with the provided options.
func NewHealth(opts ...Option) *Health {
	health := &Health{
		checks: make(map[string]CheckConfig),
	}

	for _, opt := range opts {
		opt(health)
	}

	return health
}

// WithChecks adds the checks to be run as part of the health check.
func WithChecks(checkConf ...CheckConfig) Option {
	return func(health *Health) {
		for _, conf := range checkConf {
			health.RegisterCheck(conf)
		}
	}
}

// RegisterCheck registers a check to be run as part of the health check.
func (h *Health) RegisterCheck(conf CheckConfig) {
	if conf.Timeout <= 0 {
		conf.Timeout = DefaultTimeout
	}

	h.checks[conf.Name] = conf
}

// Handler returns a http.Handler that retrieves the health status of the
// application based on the provided check functions.
func (h *Health) Handler() http.Handler {
	return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
		errGroup, ctx := errgroup.WithContext(req.Context())

		for _, check := range h.checks {
			errGroup.Go(func() error {
				ctxWithTimeout, cancel := context.WithTimeout(ctx, check.Timeout)

				defer cancel()

				checkerChan := make(chan error, 1)

				go func() {
					checkerChan <- check.CheckFn(ctxWithTimeout)
				}()

				select {
				case checkErr := <-checkerChan:
					if checkErr != nil {
						return &CheckError{name: check.Name, err: checkErr}
					}

					return nil
				case <-ctxWithTimeout.Done():
					return &TimeoutError{name: check.Name, timeElapsed: check.Timeout}
				}
			})
		}

		if err := errGroup.Wait(); err != nil {
			respW.WriteHeader(http.StatusServiceUnavailable)

			responseBody := Response{
				Error:        errorToErrorCode(err),
				ErrorMessage: err.Error(),
			}

			slog.ErrorContext(
				req.Context(),
				"health check failed",
				slog.String("path", req.URL.Path),
				slog.String("method", req.Method),
				slog.Any("err", err),
			)

			respW.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(respW).
				EncodeContext(req.Context(), responseBody); err != nil {
				respW.WriteHeader(http.StatusInternalServerError)

				slog.ErrorContext(
					req.Context(),
					"write response body",
					slog.String("path", req.URL.Path),
					slog.String("method", req.Method),
					slog.Any("err", err),
				)

				return
			}
		}
	})
}

func errorToErrorCode(err error) string {
	var (
		timeoutErr *TimeoutError
		checkErr   *CheckError
	)

	switch {
	case errors.As(err, &timeoutErr):
		return "timeout_error"
	case errors.As(err, &checkErr):
		return "check_error"
	default:
		return "internal_error"
	}
}
