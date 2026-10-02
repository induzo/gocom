package shutdown_test

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/induzo/gocom/shutdown/v2"
)

// run owns the whole application lifecycle: it creates the signal context,
// registers the cleanup hooks, validates their order, waits for a stop
// signal, and runs cleanup exactly once from a single deferred call. Startup
// and cleanup errors are both reported to the caller through errors.Join.
//
// Never call log.Fatal or os.Exit inside run or its workers: they bypass the
// deferred cleanup. Do not discard the cleanup error with a bare
// `defer s.Shutdown(...)` either.
func run(ctx context.Context) (err error) {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	s := shutdown.New(shutdown.WithGracePeriodDuration(25 * time.Second))

	defer func() {
		// Restore default signal behavior first, so a second SIGINT during
		// a slow cleanup terminates the process instead of being swallowed.
		stop()

		// ctx is already canceled when a signal arrives, so detach it:
		// hooks still get the 25s grace period as their deadline.
		err = errors.Join(err, s.Shutdown(context.WithoutCancel(ctx)))
	}()

	// A real application closes its database pool here.
	if addErr := s.Add("database", func(_ context.Context) error {
		fmt.Println("database closed")

		return nil
	}); addErr != nil {
		return fmt.Errorf("register database hook: %w", addErr)
	}

	// A real application calls srv.Shutdown(ctx) here. Before("database")
	// guarantees in-flight requests finish before the pool is closed.
	if addErr := s.Add("http server", func(_ context.Context) error {
		fmt.Println("http server stopped")

		return nil
	}, shutdown.Before("database")); addErr != nil {
		return fmt.Errorf("register http server hook: %w", addErr)
	}

	// Validate the ordering constraints before serving traffic.
	if _, hooksErr := s.Hooks(); hooksErr != nil {
		return fmt.Errorf("validate shutdown order: %w", hooksErr)
	}

	// Serve until SIGINT/SIGTERM (or the parent context) ends ctx.
	<-ctx.Done()

	return nil
}

func ExampleShutdown() {
	// A real application passes context.Background() and run returns when a
	// signal arrives. The example passes an already-canceled context so that
	// run returns immediately and the output is deterministic.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := run(ctx); err != nil {
		fmt.Println("run failed:", err)
	}

	// Output:
	// http server stopped
	// database closed
}
