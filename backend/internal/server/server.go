package server

import (
	"fmt"
	"net/http"

	"qatar-maintenance/backend/internal/config"
	"time"
)

// New builds an HTTP server without starting a listener.
// The caller owns the handler and server lifecycle.
func New(cfg config.Config, handler http.Handler) (*http.Server, error) {
	if handler == nil {
		return nil, fmt.Errorf("HTTP handler is required")
	}

	// Restrict this development foundation to local access.
	if cfg.Host != config.DefaultHost {
		return nil, fmt.Errorf("invalid server host: localhost binding required")
	}

	if cfg.Port < 1024 || cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid server port: expected 1024 to 65535")
	}

	return &http.Server{
		Addr:              cfg.Address(),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}, nil
}
