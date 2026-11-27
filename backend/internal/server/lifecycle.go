package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Run serves HTTP until startup fails or ctx is cancelled.
// Graceful shutdown waits for active requests up to shutdownTimeout.
// On shutdown failure, Close forces active connections to close.
func Run(ctx context.Context, srv *http.Server, shutdownTimeout time.Duration) error {
	if ctx == nil {
		return fmt.Errorf("server context is required")
	}
	if srv == nil {
		return fmt.Errorf("HTTP server is required")
	}
	if shutdownTimeout <= 0 || shutdownTimeout > 30*time.Second {
		return fmt.Errorf("invalid shutdown timeout: expected duration up to 30s")
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	shutdownErr := srv.Shutdown(shutdownCtx)

	if shutdownErr != nil {
		// Shutdown timed out or otherwise failed.
		// Force-close open connections before returning.
		closeErr := srv.Close()

		// Always collect the listener result to avoid leaving
		// the server goroutine unobserved.
		serveErr := <-serverErr

		result := fmt.Errorf(
			"graceful shutdown failed: %w",
			shutdownErr,
		)

		if closeErr != nil {
			result = errors.Join(
				result,
				fmt.Errorf("forced close failed: %w", closeErr),
			)
		}

		if serveErr != nil &&
			!errors.Is(serveErr, http.ErrServerClosed) {
			result = errors.Join(
				result,
				fmt.Errorf("server stopped with error: %w", serveErr),
			)
		}

		return result
	}

	if err := <-serverErr; err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server stopped with error: %w", err)
	}

	return nil
}
