package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"qatar-maintenance/backend/internal/config"
	"qatar-maintenance/backend/internal/logging"
	serverpkg "qatar-maintenance/backend/internal/server"
	"syscall"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// newHandler builds the API router with request middleware.
func newHandler() http.Handler {
	mux := http.NewServeMux()

	// Match all methods explicitly, so we control the
	// JSON error format for unsupported methods.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")

			// HEAD responses should not contain a body.
			if r.Method == http.MethodHead {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			writeAPIError(
				w,
				http.StatusMethodNotAllowed,
				ErrorCodeMethodNotAllowed,
				"HTTP method not allowed",
				requestIDFromContext(r.Context()),
			)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		response := healthResponse{
			Status:  "ok",
			Service: "qatar-maintenance-api",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("health encoding error: %v", err)
		}
	})

	// Catch unmatched routes and return consistent JSON.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		writeAPIError(
			w,
			http.StatusNotFound,
			ErrorCodeNotFound,
			"Resource not found",
			requestIDFromContext(r.Context()),
		)
	})

	return requestMiddleware(mux)
}

func buildApplication(
	lookup config.LookupEnv,
	logOutput io.Writer,
) (config.Config, *http.Server, *slog.Logger, error) {
	cfg, err := config.Load(lookup)
	if err != nil {
		return config.Config{}, nil, nil, err
	}

	logger, err := logging.New(logOutput, cfg.LogLevel)
	if err != nil {
		return config.Config{}, nil, nil, err
	}

	srv, err := serverpkg.New(cfg, newHandler())
	if err != nil {
		return config.Config{}, nil, nil, err
	}

	return cfg, srv, logger, nil
}

func main() {
	cfg, server, logger, err := buildApplication(
		os.LookupEnv,
		os.Stderr,
	)
	if err != nil {
		log.Fatal(err)
	}

	logger.Info(
		"application starting",
		"service", "qatar-maintenance-api",
		"address", server.Addr,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	if err := serverpkg.Run(
		ctx,
		server,
		cfg.ShutdownTimeout,
	); err != nil {
		logger.Error(
			"HTTP server stopped with error",
			"error", err,
		)
		os.Exit(1)
	}

	if ctx.Err() != nil {
		logger.Info("application shutdown completed")
	} else {
		logger.Info("HTTP server stopped")
	}
}
