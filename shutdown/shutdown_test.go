package shutdown

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

// hookSpec describes a hook to register through Add in table tests.
type hookSpec struct {
	name   string
	fn     func(ctx context.Context) error
	before string
	// useBefore distinguishes "no Before option" from Before("").
	useBefore bool
}

// register adds every spec through Add and fails the test on the first
// registration error.
func register(t *testing.T, s *Shutdown, specs []hookSpec) {
	t.Helper()

	for _, spec := range specs {
		var opts []HookOption
		if spec.useBefore {
			opts = append(opts, Before(spec.before))
		}

		if err := s.Add(spec.name, spec.fn, opts...); err != nil {
			t.Fatalf("Add(%q): %v", spec.name, err)
		}
	}
}

// hookNames extracts the names of hooks in order.
func hookNames(hooks []Hook) []string {
	names := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		names = append(names, hook.Name)
	}

	return names
}

// recorder appends a name to data when called, honoring nothing else.
func recorder(data *[]string, name string) func(context.Context) error {
	return func(_ context.Context) error {
		*data = append(*data, name)

		return nil
	}
}

// blockUntilDone waits for ctx to end and returns its error so the hook
// goroutine never outlives the runner.
func blockUntilDone(ctx context.Context) error {
	<-ctx.Done()

	return ctx.Err()
}

// failingHook always returns an error.
func failingHook(_ context.Context) error {
	return errors.New("dummy error")
}

func TestShutdown(t *testing.T) { //nolint:tparallel // subtests share the data slice
	t.Parallel()

	data := make([]string, 0)

	tests := []struct {
		name                string
		hooks               []hookSpec
		gracePeriodDuration time.Duration
		expectResult        []string
		expectErr           bool
	}{
		{
			name: "happy path",
			hooks: []hookSpec{
				{name: "happy1", fn: recorder(&data, "happy1")},
				{name: "happy2", fn: recorder(&data, "happy2")},
				{name: "happy3", fn: recorder(&data, "happy3")},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy3", "happy2", "happy1"},
			expectErr:           false,
		},
		{
			name: "happy path, with one before",
			hooks: []hookSpec{
				{name: "happy1", fn: recorder(&data, "happy1")},
				{name: "happy2", fn: recorder(&data, "happy2"), before: "happy3", useBefore: true},
				{name: "happy3", fn: recorder(&data, "happy3")},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy2", "happy3", "happy1"},
			expectErr:           false,
		},
		{
			// A missing target is diagnosed by Hooks, but cleanup still
			// runs every hook (the constrained one as unconstrained).
			name: "with one before which does not exist",
			hooks: []hookSpec{
				{name: "happy1", fn: recorder(&data, "happy1")},
				{
					name:      "happy2",
					fn:        recorder(&data, "happy2"),
					before:    "not exists",
					useBefore: true,
				},
				{name: "happy3", fn: recorder(&data, "happy3")},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy3", "happy2", "happy1"},
			expectErr:           true,
		},
		{
			name: "happy path, with 2 before the same target",
			hooks: []hookSpec{
				{name: "happy1", fn: recorder(&data, "happy1"), before: "happy3", useBefore: true},
				{name: "happy2", fn: recorder(&data, "happy2"), before: "happy3", useBefore: true},
				{name: "happy3", fn: recorder(&data, "happy3")},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy1", "happy2", "happy3"},
			expectErr:           false,
		},
		{
			// Cycle members are reported, run first in reverse
			// registration order, and do not prevent the others from
			// running.
			name: "before with circular dependency",
			hooks: []hookSpec{
				{name: "happy1", fn: recorder(&data, "happy1"), before: "happy2", useBefore: true},
				{name: "happy2", fn: recorder(&data, "happy2"), before: "happy1", useBefore: true},
				{name: "happy3", fn: recorder(&data, "happy3")},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy2", "happy1", "happy3"},
			expectErr:           true,
		},
		{
			name: "error path",
			hooks: []hookSpec{
				{name: "error", fn: failingHook},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{},
			expectErr:           true,
		},
		{
			name: "exceed grace period",
			hooks: []hookSpec{
				{name: "long", fn: blockUntilDone},
			},
			gracePeriodDuration: time.Millisecond,
			expectResult:        []string{},
			expectErr:           true,
		},
		{
			name: "one shutdown func fail",
			hooks: []hookSpec{
				{name: "happy1", fn: recorder(&data, "foo")},
				{name: "happy2", fn: recorder(&data, "bar")},
				{name: "error", fn: failingHook},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"bar", "foo"},
			expectErr:           true,
		},
	}

	for _, tt := range tests { //nolint:paralleltest // subtests share the data slice
		t.Run(tt.name, func(t *testing.T) {
			s := New(WithGracePeriodDuration(tt.gracePeriodDuration))
			register(t, s, tt.hooks)

			data = make([]string, 0)

			err := s.Shutdown(context.Background())
			if (err != nil) != tt.expectErr {
				t.Errorf("expect err %v but got %v", tt.expectErr, err)
			}

			if !reflect.DeepEqual(data, tt.expectResult) {
				t.Errorf("expect result: %v, got: %v", tt.expectResult, data)
			}
		})
	}
}

// TestShutdown_Direct verifies that Shutdown runs cleanup immediately, with
// no signal involved, and that a repeated call reruns the hooks: the
// package does not add idempotence.
func TestShutdown_Direct(t *testing.T) {
	t.Parallel()

	var calls int

	s := New()
	if err := s.Add("counter", func(_ context.Context) error {
		calls++

		return nil
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	for i := range 2 {
		if err := s.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown call %d: %v", i+1, err)
		}
	}

	if calls != 2 {
		t.Errorf("hook calls = %d, want 2 (Shutdown must rerun hooks)", calls)
	}
}

// TestAdd_ValidationErrors covers the three programmer-error cases that
// Add rejects with a typed error rather than silently registering a
// broken hook.
func TestAdd_ValidationErrors(t *testing.T) {
	t.Parallel()

	noopFn := func(_ context.Context) error { return nil }

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()

		s := New()
		if err := s.Add("", noopFn); !errors.Is(err, ErrEmptyHookName) {
			t.Errorf("got %v, want ErrEmptyHookName", err)
		}
	})

	t.Run("nil shutdown func", func(t *testing.T) {
		t.Parallel()

		s := New()
		if err := s.Add("name", nil); !errors.Is(err, ErrNilShutdownFunc) {
			t.Errorf("got %v, want ErrNilShutdownFunc", err)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		t.Parallel()

		s := New()
		if err := s.Add("dup", noopFn); err != nil {
			t.Fatalf("first Add: %v", err)
		}

		if err := s.Add("dup", noopFn); !errors.Is(err, ErrDuplicateHookName) {
			t.Errorf("got %v, want ErrDuplicateHookName", err)
		}

		hooks, err := s.Hooks()
		if err != nil {
			t.Fatalf("Hooks: %v", err)
		}

		if got := len(hooks); got != 1 {
			t.Errorf("Hooks len = %d, want 1 (duplicate must be dropped)", got)
		}
	})
}

// TestNew_DefaultGracePeriod asserts the documented 30s default budget.
func TestNew_DefaultGracePeriod(t *testing.T) {
	t.Parallel()

	s := New()
	if s.gracePeriodDuration != 30*time.Second {
		t.Fatalf("default grace period = %v, want 30s", s.gracePeriodDuration)
	}

	hooks, err := s.Hooks()
	if err != nil {
		t.Fatalf("Hooks: %v", err)
	}

	if len(hooks) != 0 {
		t.Fatalf("new registry must be empty, got %v", hookNames(hooks))
	}
}

// TestShutdown_PanicRecovery verifies a panicking hook does not crash the
// process: the hook's panic is reported as an error wrapping errHookPanic,
// and subsequent hooks still run.
func TestShutdown_PanicRecovery(t *testing.T) {
	t.Parallel()

	var ran []string

	s := New()
	if err := s.Add("ok-second", recorder(&ran, "ok-second")); err != nil {
		t.Fatalf("Add ok-second: %v", err)
	}

	if err := s.Add("panicker", func(_ context.Context) error {
		panic("boom")
	}); err != nil {
		t.Fatalf("Add panicker: %v", err)
	}

	err := s.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected an error from Shutdown, got nil")
	}

	if !errors.Is(err, errHookPanic) {
		t.Errorf("expected error to wrap errHookPanic, got %v", err)
	}

	// FILO: panicker was added last, so it runs first; ok-second runs after.
	if !reflect.DeepEqual(ran, []string{"ok-second"}) {
		t.Errorf("expected ok-second to still run after the panic, got %v", ran)
	}
}

// TestShutdown_CancelledContext asserts that a context that is already
// done when Shutdown is called starts no hooks and reports which hook was
// skipped together with the context error.
func TestShutdown_CancelledContext(t *testing.T) {
	t.Parallel()

	var ran []string

	s := New()
	if err := s.Add("never", recorder(&ran, "never")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Shutdown(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if len(ran) != 0 {
		t.Errorf("no hook should run with a canceled context, got %v", ran)
	}
}

// TestShutdown_DetachedContext asserts that the package does not detach
// contexts itself: a caller that wants cleanup to survive cancellation
// passes context.WithoutCancel explicitly, and hooks then observe a live
// context that carries the grace-period deadline.
func TestShutdown_DetachedContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var (
		hookErr     error
		hadDeadline bool
	)

	s := New(WithGracePeriodDuration(time.Second))
	if err := s.Add("inspect", func(hookCtx context.Context) error {
		hookErr = hookCtx.Err()
		_, hadDeadline = hookCtx.Deadline()

		return nil
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := s.Shutdown(context.WithoutCancel(ctx)); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if hookErr != nil {
		t.Errorf("hook context should be live after detachment, got %v", hookErr)
	}

	if !hadDeadline {
		t.Error("hook context should carry the grace-period deadline")
	}
}

// deferredRun mirrors the recommended application shape: one deferred
// Shutdown call aggregates its error with whatever startup returned.
func deferredRun(
	ctx context.Context,
	s *Shutdown,
	startup func(ctx context.Context) error,
) (err error) {
	defer func() {
		err = errors.Join(err, s.Shutdown(context.WithoutCancel(ctx)))
	}()

	return startup(ctx)
}

// TestShutdown_Deferred covers the deferred-cleanup contract: normal stop,
// startup failure and canceled startup each trigger exactly one cleanup
// call; startup may take longer than the grace period without consuming
// the cleanup budget; and startup and cleanup errors both survive
// aggregation.
func TestShutdown_Deferred(t *testing.T) {
	t.Parallel()

	errStartup := errors.New("startup failed")
	errCleanup := errors.New("cleanup failed")

	tests := []struct {
		name          string
		startup       func(ctx context.Context) error
		cancelBefore  bool
		hookErr       error
		wantErrs      []error
		wantHookCalls int
	}{
		{
			name:          "normal stop",
			startup:       func(_ context.Context) error { return nil },
			wantHookCalls: 1,
		},
		{
			name:          "startup failure",
			startup:       func(_ context.Context) error { return errStartup },
			wantErrs:      []error{errStartup},
			wantHookCalls: 1,
		},
		{
			name:          "canceled startup",
			startup:       blockUntilDone,
			cancelBefore:  true,
			wantErrs:      []error{context.Canceled},
			wantHookCalls: 1,
		},
		{
			// Startup sleeps far longer than the default 30s grace period;
			// the cleanup budget starts only when Shutdown is called.
			name: "startup exceeds grace period",
			startup: func(_ context.Context) error {
				time.Sleep(2 * defaultGracePeriodDuration)

				return nil
			},
			wantHookCalls: 1,
		},
		{
			name:          "startup and cleanup errors aggregate",
			startup:       func(_ context.Context) error { return errStartup },
			hookErr:       errCleanup,
			wantErrs:      []error{errStartup, errCleanup},
			wantHookCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				var hookCalls int

				s := New()
				if err := s.Add("hook", func(_ context.Context) error {
					hookCalls++

					return tt.hookErr
				}); err != nil {
					t.Fatalf("Add: %v", err)
				}

				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				if tt.cancelBefore {
					cancel()
				}

				err := deferredRun(ctx, s, tt.startup)

				if len(tt.wantErrs) == 0 && err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				for _, want := range tt.wantErrs {
					if !errors.Is(err, want) {
						t.Errorf("error %v should wrap %v", err, want)
					}
				}

				if hookCalls != tt.wantHookCalls {
					t.Errorf("hook calls = %d, want %d", hookCalls, tt.wantHookCalls)
				}
			})
		})
	}
}

// TestHooks_ExecutionOrder covers the ordering contract of Hooks: the
// returned slice is in execution order, Before adjusts FILO, and every
// invalid constraint is diagnosed while every hook is still returned
// exactly once.
func TestHooks_ExecutionOrder(t *testing.T) {
	t.Parallel()

	noop := func(_ context.Context) error { return nil }

	tests := []struct {
		name      string
		register  func(t *testing.T, s *Shutdown)
		wantOrder []string
		wantErrs  []error
	}{
		{
			name:      "no hooks",
			register:  func(_ *testing.T, _ *Shutdown) {},
			wantOrder: []string{},
		},
		{
			name: "FILO without constraints",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "a", fn: noop},
					{name: "b", fn: noop},
					{name: "c", fn: noop},
				})
			},
			wantOrder: []string{"c", "b", "a"},
		},
		{
			name: "chain of before constraints",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "a", fn: noop, before: "b", useBefore: true},
					{name: "b", fn: noop, before: "c", useBefore: true},
					{name: "c", fn: noop},
					{name: "d", fn: noop},
				})
			},
			wantOrder: []string{"d", "a", "b", "c"},
		},
		{
			name: "forward reference",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "server", fn: noop, before: "database", useBefore: true},
					{name: "database", fn: noop},
				})
			},
			wantOrder: []string{"server", "database"},
		},
		{
			name: "shared target keeps registration order",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "a", fn: noop, before: "c", useBefore: true},
					{name: "b", fn: noop, before: "c", useBefore: true},
					{name: "c", fn: noop},
				})
			},
			wantOrder: []string{"a", "b", "c"},
		},
		{
			name: "first before wins",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()

				if err := s.Add("a", noop); err != nil {
					t.Fatalf("Add a: %v", err)
				}

				if err := s.Add("b", noop); err != nil {
					t.Fatalf("Add b: %v", err)
				}

				if err := s.Add("c", noop, Before("a"), Before("b")); err != nil {
					t.Fatalf("Add c: %v", err)
				}
			},
			wantOrder: []string{"b", "c", "a"},
		},
		{
			name: "missing target",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "a", fn: noop},
					{name: "b", fn: noop, before: "ghost", useBefore: true},
				})
			},
			wantOrder: []string{"b", "a"},
			wantErrs:  []error{ErrUnknownBeforeTarget},
		},
		{
			name: "empty target",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "a", fn: noop, before: "", useBefore: true},
				})
			},
			wantOrder: []string{"a"},
			wantErrs:  []error{ErrUnknownBeforeTarget},
		},
		{
			name: "self reference",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "a", fn: noop, before: "a", useBefore: true},
					{name: "b", fn: noop},
				})
			},
			wantOrder: []string{"a", "b"},
			wantErrs:  []error{ErrCircularDependency},
		},
		{
			// Unresolved hooks (the cycle and anything depending on it)
			// are appended to the registration slice, so after the final
			// reverse they run first, in reverse registration order.
			name: "cycle with dependent",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "a", fn: noop, before: "b", useBefore: true},
					{name: "b", fn: noop, before: "a", useBefore: true},
					{name: "c", fn: noop},
					{name: "d", fn: noop, before: "a", useBefore: true},
				})
			},
			wantOrder: []string{"d", "b", "a", "c"},
			wantErrs:  []error{ErrCircularDependency},
		},
		{
			name: "mixed ordering errors",
			register: func(t *testing.T, s *Shutdown) {
				t.Helper()
				register(t, s, []hookSpec{
					{name: "a", fn: noop, before: "b", useBefore: true},
					{name: "b", fn: noop, before: "a", useBefore: true},
					{name: "c", fn: noop, before: "ghost", useBefore: true},
					{name: "d", fn: noop},
				})
			},
			wantOrder: []string{"b", "a", "d", "c"},
			wantErrs:  []error{ErrCircularDependency, ErrUnknownBeforeTarget},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := New()
			tt.register(t, s)

			hooks, err := s.Hooks()

			if len(tt.wantErrs) == 0 && err != nil {
				t.Fatalf("unexpected ordering error: %v", err)
			}

			for _, want := range tt.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("error %v should wrap %v", err, want)
				}
			}

			if got := hookNames(hooks); !reflect.DeepEqual(got, tt.wantOrder) {
				t.Errorf("order = %v, want %v", got, tt.wantOrder)
			}

			// Hooks must be a pure inspection: a second call gives the
			// same answer, and Shutdown runs exactly that order.
			again, againErr := s.Hooks()
			if !reflect.DeepEqual(hookNames(again), hookNames(hooks)) {
				t.Errorf("second Hooks call = %v, want %v", hookNames(again), hookNames(hooks))
			}

			if (againErr == nil) != (err == nil) {
				t.Errorf("second Hooks error = %v, first = %v", againErr, err)
			}

			assertShutdownRunsOrder(t, s, tt.wantOrder)
		})
	}
}

// assertShutdownRunsOrder swaps every registered function for a recorder
// and checks that Shutdown runs each hook exactly once, in the order that
// Hooks reported, without reversing it again.
func assertShutdownRunsOrder(t *testing.T, s *Shutdown, wantOrder []string) {
	t.Helper()

	ran := make([]string, 0)

	s.mutex.Lock()

	for i := range s.hooks {
		s.hooks[i].ShutdownFn = recorder(&ran, s.hooks[i].Name)
	}

	s.mutex.Unlock()

	// Ordering errors are expected for the invalid cases; callback errors
	// are impossible here, so only the run order matters.
	_ = s.Shutdown(context.Background())

	if !reflect.DeepEqual(ran, wantOrder) {
		t.Errorf("Shutdown ran %v, want %v", ran, wantOrder)
	}
}

// TestHooks_ForwardReferenceResolved asserts that Add does not validate
// dependency completeness: a missing target reported by Hooks disappears
// once the target is registered.
func TestHooks_ForwardReferenceResolved(t *testing.T) {
	t.Parallel()

	noop := func(_ context.Context) error { return nil }

	s := New()
	if err := s.Add("server", noop, Before("database")); err != nil {
		t.Fatalf("Add server: %v", err)
	}

	if _, err := s.Hooks(); !errors.Is(err, ErrUnknownBeforeTarget) {
		t.Fatalf("expected ErrUnknownBeforeTarget before the target exists, got %v", err)
	}

	if err := s.Add("database", noop); err != nil {
		t.Fatalf("Add database: %v", err)
	}

	hooks, err := s.Hooks()
	if err != nil {
		t.Fatalf("unexpected error after the target is registered: %v", err)
	}

	if got := hookNames(hooks); !reflect.DeepEqual(got, []string{"server", "database"}) {
		t.Errorf("order = %v, want [server database]", got)
	}
}

// TestHooks_Snapshot asserts that the slice returned by Hooks is
// independent of the registry: mutating its names or functions must not
// affect later inspection or cleanup.
func TestHooks_Snapshot(t *testing.T) {
	t.Parallel()

	var ran []string

	s := New()
	register(t, s, []hookSpec{
		{name: "first", fn: recorder(&ran, "first")},
		{name: "second", fn: recorder(&ran, "second")},
	})

	hooks, err := s.Hooks()
	if err != nil {
		t.Fatalf("Hooks: %v", err)
	}

	hooks[0].Name = "tampered"
	hooks[0].ShutdownFn = func(_ context.Context) error {
		ran = append(ran, "tampered")

		return nil
	}

	again, err := s.Hooks()
	if err != nil {
		t.Fatalf("Hooks: %v", err)
	}

	if got := hookNames(again); !reflect.DeepEqual(got, []string{"second", "first"}) {
		t.Errorf("registry names changed through the snapshot: %v", got)
	}

	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if !reflect.DeepEqual(ran, []string{"second", "first"}) {
		t.Errorf("Shutdown ran %v, want [second first]", ran)
	}
}

// TestShutdown_OrderingAndHookErrors asserts that an ordering error does
// not suppress cleanup: every hook still runs, and callback errors are
// aggregated with the ordering diagnostics.
func TestShutdown_OrderingAndHookErrors(t *testing.T) {
	t.Parallel()

	errBoom := errors.New("boom")

	var ran []string

	s := New()
	register(t, s, []hookSpec{
		{name: "a", fn: recorder(&ran, "a"), before: "b", useBefore: true},
		{name: "b", fn: recorder(&ran, "b"), before: "a", useBefore: true},
		{name: "c", fn: func(_ context.Context) error { return errBoom }},
	})

	err := s.Shutdown(context.Background())

	if !errors.Is(err, ErrCircularDependency) {
		t.Errorf("error %v should wrap ErrCircularDependency", err)
	}

	if !errors.Is(err, errBoom) {
		t.Errorf("error %v should wrap the callback error", err)
	}

	if !reflect.DeepEqual(ran, []string{"b", "a"}) {
		t.Errorf("ran %v, want [b a]", ran)
	}
}

// TestShutdown_OrderingErrorAndCancellation asserts that a canceled
// context does not hide ordering diagnostics, nor the other way round.
func TestShutdown_OrderingErrorAndCancellation(t *testing.T) {
	t.Parallel()

	noop := func(_ context.Context) error { return nil }

	s := New()
	register(t, s, []hookSpec{
		{name: "a", fn: noop, before: "ghost", useBefore: true},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Shutdown(ctx)

	if !errors.Is(err, ErrUnknownBeforeTarget) {
		t.Errorf("error %v should wrap ErrUnknownBeforeTarget", err)
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error %v should wrap context.Canceled", err)
	}
}

// TestShutdown_TimeoutSkipsRemaining uses the synctest fake clock to show
// that a hook exceeding the shared deadline is reported, and that the
// following hook is skipped and named rather than started.
func TestShutdown_TimeoutSkipsRemaining(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var ran []string

		s := New(WithGracePeriodDuration(time.Second))
		register(t, s, []hookSpec{
			{name: "skipped", fn: recorder(&ran, "skipped")},
			{name: "slow", fn: blockUntilDone},
		})

		err := s.Shutdown(t.Context())

		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("error %v should wrap context.DeadlineExceeded", err)
		}

		if len(ran) != 0 {
			t.Errorf("hook after the deadline must not run, got %v", ran)
		}
	})
}

func BenchmarkShutdown(b *testing.B) {
	s := New()

	for _, name := range []string{"happy1", "happy2"} {
		if err := s.Add(name, func(_ context.Context) error {
			time.Sleep(time.Millisecond)

			return nil
		}); err != nil {
			b.Fatalf("Add %s: %v", name, err)
		}
	}

	for b.Loop() {
		if err := s.Shutdown(context.Background()); err != nil {
			b.Fatalf("Shutdown: %v", err)
		}
	}
}
