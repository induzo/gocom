package contextslogger

import (
	"context"
	"flag"
	"io"
	"os"
	"testing"

	"go.uber.org/goleak"
	slog "golang.org/x/exp/slog"
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

func TestNewContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		logger *slog.Logger
	}{
		{
			name:   "basic",
			logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctxWithLogger := NewContext(context.Background(), test.logger)

			if ctxWithLogger.Value(contextKey{}) != test.logger {
				t.Errorf("NewContext failed to store logger in context")
			}
		})
	}
}

func BenchmarkNewContext(b *testing.B) {
	parentCtx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = NewContext(parentCtx, logger)
	}
}

func TestFromContext(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	loggerCtx := NewContext(context.Background(), logger)
	nologgerCtx := context.Background()

	tests := []struct {
		name           string
		ctx            context.Context //nolint:containedctx // it's a test
		expectedLogger *slog.Logger
	}{
		{
			name:           "logger in context",
			ctx:            loggerCtx,
			expectedLogger: logger,
		},
		{
			name:           "no logger in context",
			ctx:            nologgerCtx,
			expectedLogger: slog.Default(),
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			retrievedLogger := FromContext(test.ctx)

			if retrievedLogger != test.expectedLogger {
				t.Errorf("FromContext failed, expected %v, got %v", test.expectedLogger, retrievedLogger)
			}
		})
	}
}

func BenchmarkFromContext(b *testing.B) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctxWithLogger := NewContext(context.Background(), logger)
	ctxWithoutLogger := context.Background()

	b.Run("logger in context", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = FromContext(ctxWithLogger)
		}
	})

	b.Run("no logger in context", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = FromContext(ctxWithoutLogger)
		}
	})
}
