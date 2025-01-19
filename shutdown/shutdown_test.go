package shutdown

import (
	"context"
	"errors"
	"io"
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
			expectResult:        []string{"happy1", "happy2", "happy3"},
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
			textHandler := slog.NewTextHandler(io.Discard, nil)
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

func BenchmarkShutdown(b *testing.B) {
	textHandler := slog.NewTextHandler(io.Discard, nil)
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

	b.ResetTimer()

	for range b.N {
		go func() {
			time.Sleep(10 * time.Millisecond)

			syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		}()

		shutdownHandler.Listen(context.Background(), syscall.SIGINT)
	}
}
