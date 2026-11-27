package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

// startHardeningServer starts Run and waits until the actual
// HTTP listener responds. It uses only local loopback networking.
func startHardeningServer(
	t *testing.T,
	handler http.Handler,
) (*http.Server, string, context.CancelFunc, <-chan error) {
	t.Helper()

	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local TCP unavailable: %v", err)
	}

	address := reservation.Addr().String()
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}

	srv := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- Run(ctx, srv, 2*time.Second)
	}()

	t.Cleanup(func() {
		cancel()
		_ = srv.Close()
	})

	client := &http.Client{
		Timeout: 200 * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		select {
		case runErr := <-done:
			t.Fatalf("server stopped during startup: %v", runErr)
		default:
		}

		resp, reqErr := client.Get("http://" + address + "/ready")
		if reqErr == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				t.Fatalf(
					"readiness returned %d, want 204",
					resp.StatusCode,
				)
			}
			return srv, address, cancel, done
		}

		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("server did not become reachable")
	return nil, "", nil, nil
}

// An active request must finish before graceful shutdown
// completes, provided the shutdown deadline has not expired.
func TestRunWaitsForActiveRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	shutdownObserved := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusNoContent)
	})

	srv, address, cancel, done := startHardeningServer(t, mux)

	srv.RegisterOnShutdown(func() {
		close(shutdownObserved)
	})

	client := &http.Client{
		Timeout: 4 * time.Second,
	}

	requestDone := make(chan error, 1)

	go func() {
		resp, err := client.Get("http://" + address + "/work")
		if err != nil {
			requestDone <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			requestDone <- errors.New("unexpected response status")
			return
		}
		requestDone <- nil
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("active request did not start")
	}

	cancel()

	select {
	case <-shutdownObserved:
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown was not observed")
	}

	select {
	case err := <-done:
		t.Fatalf("Run returned before active request finished: %v", err)
	default:
	}

	close(release)

	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatalf("active request failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("active request did not finish")
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graceful shutdown failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not finish after active request")
	}
}

// A blocked active request must not prevent shutdown forever.
// Once the timeout expires, Run must force-close connections
// and return an error preserving context.DeadlineExceeded.
func TestRunShutdownTimeoutForcesClose(t *testing.T) {
	started := make(chan struct{})
	handlerExited := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/work", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
		close(handlerExited)
	})

	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local TCP unavailable: %v", err)
	}
	address := reservation.Addr().String()
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}

	srv := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer srv.Close()

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, srv, 150*time.Millisecond)
	}()

	readyClient := &http.Client{
		Timeout: 200 * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	deadline := time.Now().Add(3 * time.Second)
	ready := false

	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("server stopped before test: %v", err)
		default:
		}

		resp, reqErr := readyClient.Get("http://" + address + "/ready")
		if reqErr == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusNoContent {
				ready = true
				break
			}
		}

		time.Sleep(10 * time.Millisecond)
	}

	if !ready {
		t.Fatal("server did not become reachable")
	}

	requestDone := make(chan struct{}, 1)
	go func() {
		client := &http.Client{Timeout: 4 * time.Second}
		resp, err := client.Get("http://" + address + "/work")
		if err == nil {
			resp.Body.Close()
		}
		requestDone <- struct{}{}
	}()

	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked request did not start")
	}

	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected shutdown timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected deadline exceeded, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run failed to enforce shutdown timeout")
	}

	select {
	case <-handlerExited:
	case <-time.After(3 * time.Second):
		t.Fatal("forced close did not cancel request context")
	}

	select {
	case <-requestDone:
	case <-time.After(3 * time.Second):
		t.Fatal("HTTP request did not terminate")
	}
}
