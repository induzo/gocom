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
	before     *string
}

// Shutdown provides a way to listen for signals and handle shutdown of an application gracefully.
type Shutdown struct {
	hooks               []Hook
	hookNames           map[string]struct{}
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
		hookNames:           map[string]struct{}{},
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
			if hook.before != nil {
				shutdown.Add(hook.Name, hook.ShutdownFn, Before(*hook.before))

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

func Before(before string) HookOption {
	return func(hook *Hook) {
		if hook.before == nil && before != "" && before != hook.Name {
			hook.before = &before
		}
	}
}

// Add adds a shutdown hook to be run when the signal is received.
func (s *Shutdown) Add(
	name string,
	shutdownFunc func(ctx context.Context) error,
	hookOpts ...HookOption,
) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	hook := Hook{
		Name:       name,
		ShutdownFn: shutdownFunc,
	}

	for _, opt := range hookOpts {
		opt(&hook)
	}

	s.hooks = append(s.hooks, hook)
	s.hookNames[name] = struct{}{}
}

// Hooks returns a copy of the shutdown hooks, taking into account the before option
func (s *Shutdown) Hooks() []Hook {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	hooks := make([]Hook, 0, len(s.hooks))

	hooksWithValidBefore := make([]Hook, 0, len(s.hooks))

	// first append all hooks without before option
	for _, hook := range s.hooks {
		if hook.before != nil {
			if _, ok := s.hookNames[*hook.before]; ok {
				hooksWithValidBefore = append(hooksWithValidBefore, hook)

				continue
			}
		}

		hooks = append(hooks, hook)
	}

	// loop til there s no hooks with before options left
	// insert all hooks with before option
	rounds := 0

	for len(hooksWithValidBefore) > 0 && rounds <= factorial(len(hooksWithValidBefore)) {
		rounds++

		hook := hooksWithValidBefore[0]
		beforeIndex := -1

		// find the hook it should run before within the existing hooks
		for i, beforeHook := range hooks {
			if *hook.before == beforeHook.Name {
				beforeIndex = i

				break
			}
		}

		// if we haven't found the hook it should run before, skip this iteration
		if beforeIndex == -1 {
			// move it at the end
			hooksWithValidBefore = append(hooksWithValidBefore[1:], hook)

			continue
		}

		// insert the hook at the correct index, after the before hook (as it is FILO)
		hooks = append(hooks[:beforeIndex+1], append([]Hook{hook}, hooks[beforeIndex+1:]...)...)

		// remove from hooksWithValidBefore
		hooksWithValidBefore = append(hooksWithValidBefore[:0], hooksWithValidBefore[1:]...)
	}

	if len(hooksWithValidBefore) > 0 {
		// append all remaining hooks with before option at the end
		hooks = append(hooks, hooksWithValidBefore...)

		s.logger.WarnContext(
			context.Background(),
			"circular dependency detected in hooks, running them not in order",
		)
	}

	return hooks
}

// factorial calculates the factorial of n using iteration, for the worst case scenario
func factorial(n int) int {
	result := 1

	for i := 2; i <= n; i++ {
		result *= i
	}

	return result
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

	hooks := s.Hooks() //nolint:contextcheck // we are not using the context here, so it is safe to ignore

loop:
	for i := range hooks {
		hook := hooks[len(hooks)-1-i]

		s.logger.InfoContext(ctx, hook.Name+" is shutting down")

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

	s.logger.InfoContext(ctx, fmt.Sprintf("time taken for shutdown: %v", time.Since(start)))

	if sErr != nil {
		return sErr
	}

	return nil
}
