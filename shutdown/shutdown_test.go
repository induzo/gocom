package shutdown

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	leak := flag.Bool("leak", false, "use leak detector")
	flag.Parse()

	if *leak {
		goleak.VerifyTestMain(m)

		return
	}

	os.Exit(m.Run())
}

func TestShutdown(t *testing.T) { //nolint:tparallel // subtest modify same slice
	t.Parallel()

	data := make([]string, 0)

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
					ShutdownFn: func(ctx context.Context) error {
						data = append(data, "foo")

						return nil
					},
				},
				{
					Name: "happy2",
					ShutdownFn: func(ctx context.Context) error {
						data = append(data, "bar")

						return nil
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"bar", "foo"},
			expectErr:           false,
		},
		{
			name: "error path",
			hooks: []Hook{
				{
					Name: "error",
					ShutdownFn: func(ctx context.Context) error {
						return fmt.Errorf("dummy error")
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
					ShutdownFn: func(ctx context.Context) error {
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
					ShutdownFn: func(ctx context.Context) error {
						data = append(data, "foo")

						return nil
					},
				},
				{
					Name: "happy2",
					ShutdownFn: func(ctx context.Context) error {
						data = append(data, "bar")

						return nil
					},
				},
				{
					Name: "error",
					ShutdownFn: func(ctx context.Context) error {
						return fmt.Errorf("dummy error")
					},
				},
			},
			gracePeriodDuration: time.Second,
			expectResult:        []string{"bar", "foo"},
			expectErr:           true,
		},
	}

	for _, tt := range tests { //nolint:paralleltest // subtest modify same map
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			textHandler := slog.NewTextHandler(io.Discard, nil)
			logger := slog.New(textHandler)

			shutdownHandler := New(
				logger,
				WithHooks(tt.hooks),
				WithGracePeriodDuration(tt.gracePeriodDuration))

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
				ShutdownFn: func(ctx context.Context) error {
					time.Sleep(time.Millisecond)

					return nil
				},
			},
			{
				Name: "happy2",
				ShutdownFn: func(ctx context.Context) error {
					time.Sleep(time.Millisecond)

					return nil
				},
			},
		}))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		go func() {
			time.Sleep(10 * time.Millisecond)

			syscall.Kill(syscall.Getpid(), syscall.SIGINT)
		}()

		shutdownHandler.Listen(context.Background(), syscall.SIGINT)
	}
}
