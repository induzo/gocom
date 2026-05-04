package shutdown

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"syscall"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestShutdown(t *testing.T) { //nolint:tparallel // subtest modify same slice
	t.Parallel()

	data := make([]string, 0)

	ptr := func(s string) *string {
		return &s
	}

	tests := []struct {
		name                string
		hooks               []Hook
		gracePeriodDuration time.Duration
		expectResult        []string
		expectErr           bool
	}{
		{
			name: "happy path",
			hooks: []Hook{
				{
					Name: "happy1",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy1")

						return nil
					},
				},
				{
					Name: "happy2",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy2")

						return nil
					},
				},
				{
					Name: "happy3",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy3")

						return nil
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy3", "happy2", "happy1"},
			expectErr:           false,
		},
		{
			name: "happy path, with one before",
			hooks: []Hook{
				{
					Name: "happy1",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy1")

						return nil
					},
				},
				{
					Name: "happy2",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy2")

						return nil
					},
					before: ptr("happy3"),
				},
				{
					Name: "happy3",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy3")

						return nil
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy2", "happy3", "happy1"},
			expectErr:           false,
		},
		{
			name: "happy path, with one before which does not exists",
			hooks: []Hook{
				{
					Name: "happy1",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy1")

						return nil
					},
				},
				{
					Name: "happy2",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy2")

						return nil
					},
					before: ptr("not exists"),
				},
				{
					Name: "happy3",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy3")

						return nil
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy3", "happy2", "happy1"},
			expectErr:           false,
		},
		{
			name: "happy path, with 2 before",
			hooks: []Hook{
				{
					Name: "happy1",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy1")

						return nil
					},
					before: ptr("happy3"),
				},
				{
					Name: "happy2",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy2")

						return nil
					},
					before: ptr("happy3"),
				},
				{
					Name: "happy3",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy3")

						return nil
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy1", "happy2", "happy3"},
			expectErr:           false,
		},
		{
			name: "happy path, with 2 before the same",
			hooks: []Hook{
				{
					Name: "happy1",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy1")

						return nil
					},
					before: ptr("happy3"),
				},
				{
					Name: "happy2",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy2")

						return nil
					},
					before: ptr("happy3"),
				},
				{
					Name: "happy3",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy3")

						return nil
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy1", "happy2", "happy3"},
			expectErr:           false,
		},
		{
			// When the Before constraints form a cycle, the unresolvable
			// hooks are appended at the end in registration order (after
			// hooks without a constraint) and a warning is logged. The
			// order of the cycle members themselves is not guaranteed.
			name: "before with circular dependency",
			hooks: []Hook{
				{
					Name: "happy1",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy1")

						return nil
					},
					before: ptr("happy2"),
				},
				{
					Name: "happy2",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy2")

						return nil
					},
					before: ptr("happy1"),
				},
				{
					Name: "happy3",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "happy3")

						return nil
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"happy2", "happy1", "happy3"},
			expectErr:           false,
		},
		{
			name: "error path",
			hooks: []Hook{
				{
					Name: "error",
					ShutdownFn: func(_ context.Context) error {
						return errors.New("dummy error")
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{},
			expectErr:           true,
		},
		{
			name: "exceed grace period",
			hooks: []Hook{
				{
					Name: "long",
					ShutdownFn: func(_ context.Context) error {
						time.Sleep(100 * time.Millisecond)

						return nil
					},
				},
			},
			gracePeriodDuration: time.Millisecond,
			expectResult:        []string{},
			expectErr:           true,
		},
		{
			name: "one shutdown func fail",
			hooks: []Hook{
				{
					Name: "happy1",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "foo")

						return nil
					},
				},
				{
					Name: "happy2",
					ShutdownFn: func(_ context.Context) error {
						data = append(data, "bar")

						return nil
					},
				},
				{
					Name: "error",
					ShutdownFn: func(_ context.Context) error {
						return errors.New("dummy error")
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"bar", "foo"},
			expectErr:           true,
		},
	}

	for _, tt := range tests { //nolint:paralleltest // subtest modify same map
		t.Run(tt.name, func(t *testing.T) {
			textHandler := slog.DiscardHandler
			logger := slog.New(textHandler)

			shutdownHandler := New(
				logger,
				WithHooks(tt.hooks),
				WithGracePeriodDuration(tt.gracePeriodDuration),
			)

			go func() {
				time.Sleep(10 * time.Millisecond)

				syscall.Kill(syscall.Getpid(), syscall.SIGINT)
			}()

			data = make([]string, 0)

			err := shutdownHandler.Listen(context.Background(), syscall.SIGINT)

			if (err != nil) != tt.expectErr {
				t.Errorf("expect err %v but got %v", tt.expectErr, err)
			}

			if !reflect.DeepEqual(data, tt.expectResult) {
				t.Errorf("expect result: %v, got: %v", tt.expectResult, data)
			}
		})
	}
}

// TestAdd_ValidationErrors covers the three programmer-error cases that
// Add now rejects with a typed error rather than silently registering a
// broken hook.
func TestAdd_ValidationErrors(t *testing.T) {
	t.Parallel()

	noopFn := func(_ context.Context) error { return nil }

	t.Run("empty name", func(t *testing.T) {
		t.Parallel()

		s := New(slog.New(slog.DiscardHandler))
		if err := s.Add("", noopFn); !errors.Is(err, ErrEmptyHookName) {
			t.Errorf("got %v, want ErrEmptyHookName", err)
		}
	})

	t.Run("nil shutdown func", func(t *testing.T) {
		t.Parallel()

		s := New(slog.New(slog.DiscardHandler))
		if err := s.Add("name", nil); !errors.Is(err, ErrNilShutdownFunc) {
			t.Errorf("got %v, want ErrNilShutdownFunc", err)
		}
	})

	t.Run("duplicate name", func(t *testing.T) {
		t.Parallel()

		s := New(slog.New(slog.DiscardHandler))
		if err := s.Add("dup", noopFn); err != nil {
			t.Fatalf("first Add: %v", err)
		}

		if err := s.Add("dup", noopFn); !errors.Is(err, ErrDuplicateHookName) {
			t.Errorf("got %v, want ErrDuplicateHookName", err)
		}

		if got := len(s.Hooks()); got != 1 {
			t.Errorf("Hooks len = %d, want 1 (duplicate must be dropped)", got)
		}
	})
}

// TestNew_NilLogger asserts that a nil logger is replaced with
// slog.Default() so subsequent operations don't panic.
func TestNew_NilLogger(t *testing.T) {
	t.Parallel()

	s := New(nil)
	if s.logger == nil {
		t.Fatal("logger should default to slog.Default()")
	}
}

// TestListen_PanicRecovery verifies a panicking hook does not crash the
// process: the hook's panic is reported as an error wrapping errHookPanic,
// and subsequent hooks still run.
func TestListen_PanicRecovery(t *testing.T) {
	t.Parallel()

	var ran []string

	s := New(slog.New(slog.DiscardHandler))
	if err := s.Add("ok-second", func(_ context.Context) error {
		ran = append(ran, "ok-second")
		return nil
	}); err != nil {
		t.Fatalf("Add ok-second: %v", err)
	}

	if err := s.Add("panicker", func(_ context.Context) error {
		panic("boom")
	}); err != nil {
		t.Fatalf("Add panicker: %v", err)
	}

	go func() {
		time.Sleep(10 * time.Millisecond)

		_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	err := s.Listen(context.Background(), syscall.SIGINT)
	if err == nil {
		t.Fatal("expected an error from Listen, got nil")
	}

	if !errors.Is(err, errHookPanic) {
		t.Errorf("expected error to wrap errHookPanic, got %v", err)
	}

	// FILO: panicker was added last, so it runs first; ok-second runs after.
	if len(ran) != 1 || ran[0] != "ok-second" {
		t.Errorf("expected ok-second to still run after the panic, got %v", ran)
	}
}

// TestWithHooks_SkipsInvalid asserts WithHooks logs and skips hooks that
// fail Add validation, while still registering valid ones.
func TestWithHooks_SkipsInvalid(t *testing.T) {
	t.Parallel()

	noopFn := func(_ context.Context) error { return nil }

	s := New(slog.New(slog.DiscardHandler), WithHooks([]Hook{
		{Name: "", ShutdownFn: noopFn},  // empty name -> skipped
		{Name: "a", ShutdownFn: nil},    // nil fn -> skipped
		{Name: "b", ShutdownFn: noopFn}, // valid
		{Name: "b", ShutdownFn: noopFn}, // duplicate -> skipped
		{Name: "c", ShutdownFn: noopFn}, // valid
	}))

	hooks := s.Hooks()
	if len(hooks) != 2 {
		t.Fatalf("expected 2 valid hooks, got %d (%v)", len(hooks), hooks)
	}

	gotNames := []string{hooks[0].Name, hooks[1].Name}

	wantNames := []string{"b", "c"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Errorf("hook names = %v, want %v", gotNames, wantNames)
	}
}

func BenchmarkShutdown(b *testing.B) {
	textHandler := slog.DiscardHandler
	logger := slog.New(textHandler)

	shutdownHandler := New(
		logger,
		WithHooks([]Hook{
			{
				Name: "happy1",
				ShutdownFn: func(_ context.Context) error {
					time.Sleep(time.Millisecond)

					return nil
				},
			},
			{
				Name: "happy2",
				ShutdownFn: func(_ context.Context) error {
					time.Sleep(time.Millisecond)

					return nil
				},
			},
		}))

	for b.Loop() {
		go func() {
			time.Sleep(10 * time.Millisecond)

			syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		}()

		shutdownHandler.Listen(context.Background(), syscall.SIGINT)
	}
}
