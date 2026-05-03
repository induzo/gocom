// Package health provides an HTTP handler that runs a set of named check
// functions in parallel and returns 200 OK or 503 Service Unavailable
// depending on the aggregated outcome.
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
	// HealthEndpoint is a suggested URL path for mounting the health handler.
	// Callers may mount it anywhere; this constant is provided for convenience.
	HealthEndpoint = "/sys/health"
)

const (
	// DefaultTimeout is applied to a CheckConfig that does not set its own
	// non-zero Timeout.
	DefaultTimeout = 3 * time.Second
)

const (
	errorCodeTimeout  = "timeout_error"
	errorCodeCheck    = "check_error"
	errorCodeInternal = "internal_error"
)

// errCheckPanic is the underlying error wrapped when a CheckFn panics.
var errCheckPanic = errors.New("check panicked")

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
	// Name is a stable identifier for the check; surfaced in errors and logs.
	Name string

	// CheckFn is the probe function. Implementations MUST honor ctx and
	// return promptly when it is cancelled, otherwise the goroutine running
	// the check will outlive the request and accumulate per probe.
	CheckFn func(ctx context.Context) error

	// Timeout bounds a single invocation of CheckFn. When zero or negative,
	// DefaultTimeout is applied.
	Timeout time.Duration
}

// Response is the JSON body returned by Handler on a 503 response. A 200
// response has an empty body.
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
//
// RegisterCheck is intended to be called during construction (typically via
// WithChecks) before Handler() is wired into a server. It is not safe to
// call concurrently with requests that hit Handler().
func (h *Health) RegisterCheck(conf CheckConfig) {
	if conf.Timeout <= 0 {
		conf.Timeout = DefaultTimeout
	}

	h.checks[conf.Name] = conf
}

// Handler returns a http.Handler that retrieves the health status of the
// application based on the provided check functions. On success the handler
// writes 200 OK with an empty body; on failure it writes 503 Service
// Unavailable with a JSON [Response] body.
func (h *Health) Handler() http.Handler {
	return http.HandlerFunc(func(respW http.ResponseWriter, req *http.Request) {
		errGroup, ctx := errgroup.WithContext(req.Context())

		for _, check := range h.checks {
			errGroup.Go(func() error {
				return runCheck(ctx, check)
			})
		}

		err := errGroup.Wait()
		if err == nil {
			return
		}

		writeFailure(respW, req, err)
	})
}

// runCheck executes a single CheckConfig under its own timeout. It recovers
// panics from CheckFn so a misbehaving check does not crash the server.
func runCheck(parent context.Context, check CheckConfig) error {
	ctxWithTimeout, cancel := context.WithTimeout(parent, check.Timeout)
	defer cancel()

	checkerChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				checkerChan <- fmt.Errorf("%w: %v", errCheckPanic, r)
			}
		}()

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
}

// writeFailure renders the 503 response. Headers (including Cache-Control:
// no-store) are set before the status code is committed so they are not
// silently dropped on the wire.
func writeFailure(respW http.ResponseWriter, req *http.Request, err error) {
	code := errorToErrorCode(err)
	name := failedCheckName(err)

	respW.Header().Set("Content-Type", "application/json")
	respW.Header().Set("Cache-Control", "no-store")
	respW.WriteHeader(http.StatusServiceUnavailable)

	slog.ErrorContext(
		req.Context(),
		"health check failed",
		slog.String("path", req.URL.Path),
		slog.String("method", req.Method),
		slog.String("check", name),
		slog.String("error_code", code),
		slog.String("err", err.Error()),
	)

	body := Response{
		Error:        code,
		ErrorMessage: err.Error(),
	}

	if encErr := json.NewEncoder(respW).EncodeContext(req.Context(), body); encErr != nil {
		slog.ErrorContext(
			req.Context(),
			"write response body",
			slog.String("path", req.URL.Path),
			slog.String("method", req.Method),
			slog.String("err", encErr.Error()),
		)
	}
}

func failedCheckName(err error) string {
	var (
		timeoutErr *TimeoutError
		checkErr   *CheckError
	)

	switch {
	case errors.As(err, &timeoutErr):
		return timeoutErr.name
	case errors.As(err, &checkErr):
		return checkErr.name
	default:
		return ""
	}
}

func errorToErrorCode(err error) string {
	var (
		timeoutErr *TimeoutError
		checkErr   *CheckError
	)

	switch {
	case errors.As(err, &timeoutErr):
		return errorCodeTimeout
	case errors.As(err, &checkErr):
		return errorCodeCheck
	default:
		return errorCodeInternal
	}
}
