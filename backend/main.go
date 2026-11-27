package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

type healthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// loadPort validates the optional local development port.
// An empty value uses port 8080.
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

func newHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")

		response := healthResponse{
			Status:  "ok",
			Service: "qatar-maintenance-api",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("health encoding error: %v", err)
		}
	})

	return mux
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
	port, err := loadPort(os.Getenv("QMM_PORT"))
	if err != nil {
		log.Fatal(err)
	}

	server := newServer(port)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error: ", err)
		}
		return

	case <-ctx.Done():
		log.Println("Shutdown signal received")
	}

	stop()

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)

		if closeErr := server.Close(); closeErr != nil {
			log.Printf("forced close failed: %v", closeErr)
		}
	}

	if err := <-serverErr; err != nil &&
		!errors.Is(err, http.ErrServerClosed) {
		log.Printf("server stopped with error: %v", err)
	}

	log.Println("Qatar Maintenance API stopped")
}
