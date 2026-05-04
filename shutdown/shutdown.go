// Package shutdown provides a small primitive for gracefully shutting down
// an application: register named hooks, wait for an OS signal, then run the
// hooks in FILO (last-registered, first-run) order under a shared grace
// period, with optional Before constraints that adjust ordering.
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

// Sentinel errors returned by Add when a hook is rejected.
var (
	// ErrEmptyHookName is returned when a hook is registered without a name.
	ErrEmptyHookName = errors.New("hook name must not be empty")

	// ErrNilShutdownFunc is returned when a hook is registered without a
	// shutdown function.
	ErrNilShutdownFunc = errors.New("hook shutdown function must not be nil")

	// ErrDuplicateHookName is returned when a hook is registered with a
	// name that is already in use. Each name must be unique within a
	// Shutdown instance.
	ErrDuplicateHookName = errors.New("hook with this name is already registered")
)

// errHookPanic is the underlying error wrapped when a hook panics during
// Listen.
var errHookPanic = errors.New("hook panicked")

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

// New returns a new Shutdown with the provided options. If logger is nil,
// slog.Default() is used.
func New(logger *slog.Logger, opts ...Option) *Shutdown {
	if logger == nil {
		logger = slog.Default()
	}

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

// WithHooks adds the hooks to be run as part of the graceful shutdown. Any
// hook that fails validation in Add (empty name, nil shutdown function,
// duplicate name) is logged at warning level and skipped; the remaining
// hooks are still registered.
//
// Note: Hook.before is unexported, so external callers using WithHooks
// cannot express a Before constraint. To use Before, register hooks via
// Add directly.
func WithHooks(hooks []Hook) Option {
	return func(shutdown *Shutdown) {
		for _, hook := range hooks {
			var err error

			if hook.before != nil {
				err = shutdown.Add(hook.Name, hook.ShutdownFn, Before(*hook.before))
			} else {
				err = shutdown.Add(hook.Name, hook.ShutdownFn)
			}

			if err != nil {
				shutdown.logger.WarnContext(
					context.Background(),
					"skipped shutdown hook",
					slog.String("name", hook.Name),
					slog.String("err", err.Error()),
				)
			}
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

// HookOption configures a Hook at registration time.
type HookOption func(*Hook)

// Before declares that the hook should run before the named hook during
// Listen. Because Listen iterates the registered slice in reverse (FILO),
// "run before X" means "ordered after X in the slice". If the named target
// is empty, equal to the hook's own name, or unknown at registration time,
// Before is a no-op.
func Before(before string) HookOption {
	return func(hook *Hook) {
		if hook.before == nil && before != "" && before != hook.Name {
			hook.before = &before
		}
	}
}

// Add registers a shutdown hook. Returns ErrEmptyHookName if name is empty,
// ErrNilShutdownFunc if shutdownFunc is nil, or ErrDuplicateHookName if a
// hook with this name is already registered.
func (s *Shutdown) Add(
	name string,
	shutdownFunc func(ctx context.Context) error,
	hookOpts ...HookOption,
) error {
	if name == "" {
		return ErrEmptyHookName //nolint:wrapcheck // sentinel returned as-is for errors.Is.
	}

	if shutdownFunc == nil {
		return ErrNilShutdownFunc //nolint:wrapcheck // sentinel returned as-is for errors.Is.
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, exists := s.hookNames[name]; exists {
		return ErrDuplicateHookName //nolint:wrapcheck // sentinel returned as-is for errors.Is.
	}

	hook := Hook{
		Name:       name,
		ShutdownFn: shutdownFunc,
	}

	for _, opt := range hookOpts {
		opt(&hook)
	}

	s.hooks = append(s.hooks, hook)
	s.hookNames[name] = struct{}{}

	return nil
}

// Hooks returns the registered shutdown hooks ordered for execution. Hooks
// without a Before constraint keep registration order; Before constraints
// are applied so that each constrained hook is positioned to run before
// its named target during Listen's reverse iteration. If a circular
// dependency is detected, the unresolved hooks are appended at the end and
// a warning is logged.
func (s *Shutdown) Hooks() []Hook {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	hooks := make([]Hook, 0, len(s.hooks))
	hooksWithValidBefore := make([]Hook, 0, len(s.hooks))

	// First, place all hooks without a (resolvable) before constraint.
	for _, hook := range s.hooks {
		if hook.before != nil {
			if _, ok := s.hookNames[*hook.before]; ok {
				hooksWithValidBefore = append(hooksWithValidBefore, hook)

				continue
			}
		}

		hooks = append(hooks, hook)
	}

	// Then, place the constrained hooks. Each pass tries to insert each
	// pending hook after its target; if a full pass makes no progress,
	// the remaining set has a circular or unresolvable dependency.
	for len(hooksWithValidBefore) > 0 {
		var madeProgress bool

		hooksWithValidBefore, hooks, madeProgress = placeHooksRound(hooksWithValidBefore, hooks)
		if !madeProgress {
			break
		}
	}

	if len(hooksWithValidBefore) > 0 {
		// Append remaining (unresolvable) hooks at the end and warn.
		hooks = append(hooks, hooksWithValidBefore...)

		s.logger.WarnContext(
			context.Background(),
			"circular dependency detected in hooks, running them not in order",
		)
	}

	return hooks
}

// placeHooksRound walks pending once and inserts each hook whose Before
// target is present in placed at the correct index. Returns hooks that
// could not be placed, the updated placed slice, and whether any insertion
// happened in this round.
func placeHooksRound(pending, placed []Hook) ([]Hook, []Hook, bool) {
	next := pending[:0:0]

	madeProgress := false

	for _, hook := range pending {
		beforeIndex := indexOfHook(placed, *hook.before)
		if beforeIndex == -1 {
			next = append(next, hook)

			continue
		}

		// Insert right after the target so it runs before the target
		// during the reverse iteration in Listen.
		placed = append(placed[:beforeIndex+1], append([]Hook{hook}, placed[beforeIndex+1:]...)...)
		madeProgress = true
	}

	return next, placed, madeProgress
}

// indexOfHook returns the index of name in hooks, or -1 if not present.
func indexOfHook(hooks []Hook, name string) int {
	for i, h := range hooks {
		if h.Name == name {
			return i
		}
	}

	return -1
}

// Listen waits for the signals provided and executes each shutdown hook
// sequentially in FILO order. It will immediately stop and return once the
// grace period has passed.
//
// Hooks must honor the ctx passed to them and return promptly when it is
// cancelled; a hook that ignores ctx will leave its goroutine running
// after Listen returns when the grace period is exceeded.
func (s *Shutdown) Listen(ctx context.Context, signals ...os.Signal) error {
	signalCtx, stopSignalCtx := signal.NotifyContext(ctx, signals...)
	defer stopSignalCtx()

	<-signalCtx.Done()

	start := time.Now()

	// Derive the shutdown deadline from the caller's ctx (not signalCtx) so
	// the signal-cancellation does not propagate into hooks; they should
	// observe a budget that times out only on the grace period.
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, s.gracePeriodDuration)
	defer shutdownCancel()

	var sErr error

	hooks := s.Hooks() //nolint:contextcheck // Hooks() does not need ctx; it only inspects already-registered state.

loop:
	for i := range hooks {
		hook := hooks[len(hooks)-1-i]

		s.logger.InfoContext(ctx, hook.Name+" is shutting down")

		errChan := make(chan error, 1)

		// Run the hook in a goroutine so we can race it against the
		// shutdown deadline. Recover panics so a single bad hook doesn't
		// take the process down.
		go func() {
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("%w: %v", errHookPanic, r)
				}
			}()

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

	return sErr
}
