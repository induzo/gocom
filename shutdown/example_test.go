package shutdown_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"syscall"
	"time"

	"github.com/induzo/gocom/shutdown"
)

//nolint:testableexamples // do not have testable output
func ExampleShutdown() {
	textHandler := slog.DiscardHandler
	logger := slog.New(textHandler)

	shutdownHandler := shutdown.New(
		logger,
		shutdown.WithHooks(
			[]shutdown.Hook{
				{
					Name: "do something",
					ShutdownFn: func(_ context.Context) error {
						return nil
					},
				},
			},
		),
		shutdown.WithGracePeriodDuration(time.Second))

	var srv http.Server

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server listen and serve: %s", err)
		}
	}()

	shutdownHandler.Add("http server", func(ctx context.Context) error {
		if err := srv.Shutdown(ctx); err != nil {
			return fmt.Errorf("http server shutdown: %w", err)
		}

		return nil
	})

	if err := shutdownHandler.Listen(
		context.Background(),
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT); err != nil {
		log.Fatalf("graceful shutdown failed: %s. forcing exit.", err)
	}
}
