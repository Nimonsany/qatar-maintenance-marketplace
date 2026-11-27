package server

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRunRejectsInvalidArguments(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		server  *http.Server
		timeout time.Duration
	}{
		{
			name:    "nil context",
			ctx:     nil,
			server:  &http.Server{},
			timeout: 5 * time.Second,
		},
		{
			name:    "nil server",
			ctx:     context.Background(),
			server:  nil,
			timeout: 5 * time.Second,
		},
		{
			name:    "zero timeout",
			ctx:     context.Background(),
			server:  &http.Server{},
			timeout: 0,
		},
		{
			name:    "negative timeout",
			ctx:     context.Background(),
			server:  &http.Server{},
			timeout: -time.Second,
		},
		{
			name:    "timeout above 30 seconds",
			ctx:     context.Background(),
			server:  &http.Server{},
			timeout: 31 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Run(tt.ctx, tt.server, tt.timeout)
			if err == nil {
				t.Fatal("Run() accepted invalid arguments")
			}
		})
	}
}

func TestRunRejectsOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local TCP unavailable: %v", err)
	}
	defer listener.Close()

	srv := &http.Server{
		Addr:              listener.Addr().String(),
		Handler:           http.NotFoundHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	err = Run(context.Background(), srv, time.Second)
	if err == nil {
		t.Fatal("Run() should fail when port is occupied")
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	// Reserve an unused local port, then release it
	// before Run starts its listener.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local TCP unavailable: %v", err)
	}

	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	srv := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- Run(ctx, srv, 2*time.Second)
	}()

	// Wait for the server to become reachable.
	client := &http.Client{
		Timeout: 100 * time.Millisecond,
	}

	deadline := time.Now().Add(3 * time.Second)
	started := false

	for time.Now().Before(deadline) {
		select {
		case runErr := <-done:
			t.Fatalf("server stopped before cancellation: %v", runErr)
		default:
		}

		response, requestErr := client.Get("http://" + address + "/health")
		if requestErr == nil {
			response.Body.Close()

			if response.StatusCode != http.StatusNoContent {
				t.Fatalf("unexpected status: %d", response.StatusCode)
			}

			started = true
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if !started {
		cancel()
		t.Fatal("server did not become reachable")
	}

	// Trigger the graceful shutdown.
	cancel()

	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("graceful shutdown failed: %v", runErr)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
}

func TestRunRejectsTimeoutAboveLimit(t *testing.T) {
	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.NotFoundHandler(),
	}

	err := Run(
		context.Background(),
		srv,
		31*time.Second,
	)

	if err == nil {
		t.Fatal("expected shutdown timeout validation error")
	}

	if !strings.Contains(err.Error(), "shutdown timeout") {
		t.Errorf("unexpected error: %v", err)
	}
}
