package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"qatar-maintenance/backend/internal/config"
	"qatar-maintenance/backend/internal/logging"
	serverpkg "qatar-maintenance/backend/internal/server"
	"strconv"
	"syscall"
	"time"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// loadPort validates the optional local development port.
func loadPort(value string) (int, error) {
	if value == "" {
		return 8080, nil
	}

	port, err := strconv.Atoi(value)
	if err != nil || port < 1024 || port > 65535 {
		return 0, fmt.Errorf(
			"invalid QMM_PORT: expected integer from 1024 to 65535",
		)
	}

	return port, nil
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

func newServer(port int) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		Handler:           newHandler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
}

func main() {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		log.Fatal(err)
	}

	logger, err := logging.New(os.Stderr, cfg.LogLevel)
	if err != nil {
		log.Fatal(err)
	}

	server, err := serverpkg.New(cfg, newHandler())
	if err != nil {
		logger.Error("HTTP server initialization failed", "error", err)
		os.Exit(1)
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
