package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Run starts the HTTP server and stops it when ctx is cancelled.
// The supplied context should be controlled by the application.
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

	if err := srv.Shutdown(shutdownCtx); err != nil {
		closeErr := srv.Close()
		if closeErr != nil {
			return errors.Join(
				fmt.Errorf("graceful shutdown failed: %w", err),
				fmt.Errorf("forced close failed: %w", closeErr),
			)
		}
		return fmt.Errorf("graceful shutdown failed: %w", err)
	}

	if err := <-serverErr; err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server stopped with error: %w", err)
	}

	return nil
}
