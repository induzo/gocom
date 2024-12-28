// This package allows you to gracefully shutdown your app.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"
)

const defaultGracePeriodDuration = 30 * time.Second

// Hook is a shutdown hook that will be called when signal is received.
type Hook struct {
	Name       string
	ShutdownFn func(ctx context.Context) error
	after      *string
}

// Shutdown provides a way to listen for signals and handle shutdown of an application gracefully.
type Shutdown struct {
	hooks               []Hook
	mutex               *sync.Mutex
	logger              *slog.Logger
	gracePeriodDuration time.Duration
}

// Option is the options type to configure Shutdown.
type Option func(*Shutdown)

// New returns a new Shutdown with the provided options.
func New(logger *slog.Logger, opts ...Option) *Shutdown {
	shutdown := &Shutdown{
		hooks:               []Hook{},
		mutex:               &sync.Mutex{},
		logger:              logger,
		gracePeriodDuration: defaultGracePeriodDuration,
	}

	for _, opt := range opts {
		opt(shutdown)
	}

	return shutdown
}

// WithHooks adds the hooks to be run as part of the graceful shutdown.
func WithHooks(hooks []Hook) Option {
	return func(shutdown *Shutdown) {
		for _, hook := range hooks {
			if hook.after != nil {
				shutdown.Add(hook.Name, hook.ShutdownFn, After(*hook.after))

				continue
			}

			shutdown.Add(hook.Name, hook.ShutdownFn)
		}
	}
}

// WithGracePeriodDuration sets the grace period for all shutdown hooks to finish running.
// If not used, the default grace period is 30s.
func WithGracePeriodDuration(gracePeriodDuration time.Duration) Option {
	return func(shutdown *Shutdown) {
		shutdown.gracePeriodDuration = gracePeriodDuration
	}
}

type HookOption func(*Hook)

func After(after string) HookOption {
	return func(hook *Hook) {
		hook.after = &after
	}
}

// Add adds a shutdown hook to be run when the signal is received.
func (s *Shutdown) Add(name string, shutdownFunc func(ctx context.Context) error, hookOpts ...HookOption) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	hook := Hook{
		Name:       name,
		ShutdownFn: shutdownFunc,
	}

	for _, opt := range hookOpts {
		opt(&hook)
	}

	// find the place to insert the hook after the after hook if != nil
	if hook.after != nil {
		for i, h := range s.hooks {
			if h.Name == *hook.after {
				// we insert the hook before the hook we found, as we shutdown in FILO order
				s.hooks = append(s.hooks[:i], append([]Hook{hook}, s.hooks[i:]...)...)

				return
			}
		}
	}

	s.hooks = append(s.hooks, hook)
}

// Hooks returns a copy of the shutdown hooks.
func (s *Shutdown) Hooks() []Hook {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	hooks := make([]Hook, 0, len(s.hooks))
	hooks = append(hooks, s.hooks...)

	return hooks
}

// Listen waits for the signals provided and executes each shutdown hook sequentially in FILO order.
// It will immediately stop and return once the grace period has passed.
func (s *Shutdown) Listen(ctx context.Context, signals ...os.Signal) error {
	signalCtx, stopSignalCtx := signal.NotifyContext(ctx, signals...)
	defer stopSignalCtx()

	<-signalCtx.Done()

	start := time.Now()

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, s.gracePeriodDuration)
	defer shutdownCancel()

	var sErr error

	hooks := s.Hooks()

loop:
	for i := range hooks {
		hook := hooks[len(hooks)-1-i]

		s.logger.Info(hook.Name + " is shutting down")

		errChan := make(chan error, 1)

		// To check the context timeout, we run shutdown func in goroutine. But it still
		// waits for getting the result from errChan before execute the next one.
		go func() {
			errChan <- hook.ShutdownFn(shutdownCtx)
		}()

		select {
		case <-shutdownCtx.Done():
			sErr = errors.Join(
				sErr,
				fmt.Errorf(
					"%s did not shutdown within grace period of %v: %w",
					hook.Name, s.gracePeriodDuration, shutdownCtx.Err(),
				),
			)

			break loop
		case err := <-errChan:
			if err != nil {
				sErr = errors.Join(sErr, fmt.Errorf("%s shutdown error: %w", hook.Name, err))
			}
		}
	}

	s.logger.Info(fmt.Sprintf("time taken for shutdown: %v", time.Since(start)))

	if sErr != nil {
		return sErr
	}

	return nil
}
