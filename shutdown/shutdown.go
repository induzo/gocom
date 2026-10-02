// Package shutdown provides a small, logger-free primitive for gracefully
// shutting down an application: register named hooks, inspect their
// execution order, then run them sequentially in FILO (last-registered,
// first-run) order under one shared grace period, with optional Before
// constraints that adjust ordering.
//
// The package does not handle signals. The application owns signal handling
// (typically through signal.NotifyContext) and calls Shutdown exactly once,
// usually from a single deferred call:
//
//	s := shutdown.New(shutdown.WithGracePeriodDuration(25 * time.Second))
//	defer func() {
//		err = errors.Join(err, s.Shutdown(context.WithoutCancel(ctx)))
//	}()
//
// Hooks must cooperate with cancellation: the runner cannot terminate a hook
// goroutine that ignores its context once the shared deadline has passed.
package shutdown

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
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

// Sentinel errors returned by Hooks (and therefore Shutdown) when the
// Before constraints cannot be satisfied.
var (
	// ErrUnknownBeforeTarget is returned when a hook declares Before on a
	// name that is not registered. The hook is treated as unconstrained.
	ErrUnknownBeforeTarget = errors.New("before target is not registered")

	// ErrCircularDependency is returned when Before constraints form a
	// cycle. The unresolved hooks (cycle members and hooks depending on
	// them) run first, in reverse registration order.
	ErrCircularDependency = errors.New("circular dependency between hooks")
)

// errHookPanic is the underlying error wrapped when a hook panics during
// Shutdown.
var errHookPanic = errors.New("hook panicked")

// Hook is a named shutdown hook executed by Shutdown.
type Hook struct {
	Name       string
	ShutdownFn func(ctx context.Context) error
	before     *string
}

// Shutdown is a registry of named hooks that are run sequentially, under
// one shared deadline, when Shutdown is called.
type Shutdown struct {
	hooks               []Hook
	hookNames           map[string]struct{}
	mutex               *sync.Mutex
	gracePeriodDuration time.Duration
}

// Option is the options type to configure Shutdown.
type Option func(*Shutdown)

// New returns an empty registry with the provided options applied. The
// default grace period is 30s.
func New(opts ...Option) *Shutdown {
	shutdown := &Shutdown{
		hooks:               []Hook{},
		hookNames:           map[string]struct{}{},
		mutex:               &sync.Mutex{},
		gracePeriodDuration: defaultGracePeriodDuration,
	}

	for _, opt := range opts {
		opt(shutdown)
	}

	return shutdown
}

// WithGracePeriodDuration sets the shared budget for all shutdown hooks to
// finish running. If not used, the default grace period is 30s. A zero or
// negative duration makes the deadline expire immediately, so Shutdown
// skips every hook and reports the first one with context.DeadlineExceeded.
func WithGracePeriodDuration(gracePeriodDuration time.Duration) Option {
	return func(shutdown *Shutdown) {
		shutdown.gracePeriodDuration = gracePeriodDuration
	}
}

// HookOption configures a Hook at registration time.
type HookOption func(*Hook)

// Before declares that the hook must run before the named hook. The target
// may be registered later: Add does not validate it, Hooks does. An empty
// or unknown target is reported by Hooks as ErrUnknownBeforeTarget; a hook
// naming itself is reported as ErrCircularDependency. If several Before
// options are supplied, the first one wins.
func Before(before string) HookOption {
	return func(hook *Hook) {
		if hook.before == nil {
			hook.before = &before
		}
	}
}

// Add registers a shutdown hook. Returns ErrEmptyHookName if name is empty,
// ErrNilShutdownFunc if shutdownFunc is nil, or ErrDuplicateHookName if a
// hook with this name is already registered. Dependency completeness is
// not checked here; call Hooks after all registrations to validate it.
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

// Hooks returns an independent snapshot of the registered hooks in the
// exact order Shutdown will run them, together with any ordering
// diagnostics. Hooks without a Before constraint run in reverse
// registration order (FILO); a constrained hook is moved to run right
// before its target, and hooks sharing a target keep their registration
// order.
//
// Every registered hook is returned exactly once, even on error. A hook
// whose target is missing is reported with ErrUnknownBeforeTarget and run as
// if unconstrained. Hooks that cannot be placed because of a cycle are
// reported with ErrCircularDependency and run first, in reverse
// registration order. Multiple diagnostics are joined with errors.Join and
// remain matchable with errors.Is.
//
// Hooks never executes callbacks or modifies registration state, and
// mutating the returned slice does not affect later cleanup.
func (s *Shutdown) Hooks() ([]Hook, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var oErr error

	placed := make([]Hook, 0, len(s.hooks))
	pending := make([]Hook, 0, len(s.hooks))

	// First, place all hooks without a (resolvable) before constraint, in
	// registration order.
	for _, hook := range s.hooks {
		if hook.before == nil {
			placed = append(placed, hook)

			continue
		}

		if _, ok := s.hookNames[*hook.before]; !ok {
			oErr = errors.Join(oErr, fmt.Errorf(
				"hook %q before %q: %w", hook.Name, *hook.before, ErrUnknownBeforeTarget,
			))

			placed = append(placed, hook)

			continue
		}

		pending = append(pending, hook)
	}

	// Then, place the constrained hooks. Each pass tries to insert each
	// pending hook after its target; if a full pass makes no progress,
	// the remaining set has a circular dependency.
	for len(pending) > 0 {
		var madeProgress bool

		pending, placed, madeProgress = placeHooksRound(pending, placed)
		if !madeProgress {
			break
		}
	}

	if len(pending) > 0 {
		names := make([]string, 0, len(pending))
		for _, hook := range pending {
			names = append(names, hook.Name)
		}

		oErr = errors.Join(
			oErr,
			fmt.Errorf("%w: %s", ErrCircularDependency, strings.Join(names, ", ")),
		)

		placed = append(placed, pending...)
	}

	// placed is in registration order (adjusted by Before); execution is
	// the reverse of it.
	slices.Reverse(placed)

	return placed, oErr
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
		// once the slice is reversed into execution order.
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

// Shutdown runs every registered hook sequentially, in the order returned
// by Hooks, under one deadline derived from ctx and the configured grace
// period. It returns the aggregate of: ordering diagnostics from Hooks,
// callback failures and recovered panics (each wrapped with the hook name),
// and a context error naming the first hook that was not started because
// the shared context had already ended.
//
// Ordering errors do not abort cleanup. A hook that panics does not crash
// the process or prevent later hooks from running. Hooks must honor the ctx
// passed to them: a hook that ignores it leaves its goroutine running after
// Shutdown returns once the deadline is exceeded.
//
// Shutdown is not idempotent: each call reruns the hooks. The caller owns
// the exactly-once rule, typically with a single deferred call. If the
// application context may already be canceled when cleanup starts, pass
// context.WithoutCancel(ctx) explicitly.
func (s *Shutdown) Shutdown(ctx context.Context) error {
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, s.gracePeriodDuration)
	defer shutdownCancel()

	hooks, sErr := s.Hooks()

	for _, hook := range hooks {
		if err := shutdownCtx.Err(); err != nil {
			return errors.Join(
				sErr,
				fmt.Errorf("%s skipped, shutdown context is done: %w", hook.Name, err),
			)
		}

		if err := s.runHook(shutdownCtx, hook); err != nil {
			sErr = errors.Join(sErr, err)
		}
	}

	return sErr
}

// runHook runs one hook in a goroutine, racing it against the shared
// shutdown deadline, and recovers panics so a single bad hook does not take
// the process down.
func (s *Shutdown) runHook(ctx context.Context, hook Hook) error {
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("%w: %v", errHookPanic, r)
			}
		}()

		errChan <- hook.ShutdownFn(ctx)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf(
			"%s did not finish before the shutdown context ended (grace period %v): %w",
			hook.Name, s.gracePeriodDuration, ctx.Err(),
		)
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("%s shutdown error: %w", hook.Name, err)
		}

		return nil
	}
}
