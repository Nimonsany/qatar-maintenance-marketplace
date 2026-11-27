package server

import (
	"net/http"
	"testing"
	"time"

	"qatar-maintenance/backend/internal/config"
)

func validTestConfig() config.Config {
	return config.Config{
		Host:            "127.0.0.1",
		Port:            8080,
		LogLevel:        "INFO",
		ShutdownTimeout: 5 * time.Second,
	}
}

func TestNewServerConfiguration(t *testing.T) {
	cfg := validTestConfig()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	srv, err := New(cfg, handler)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	if srv.Addr != "127.0.0.1:8080" {
		t.Errorf("Addr = %q, want 127.0.0.1:8080", srv.Addr)
	}

	if srv.Handler == nil {
		t.Fatal("server handler must not be nil")
	}

	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 5s", srv.ReadHeaderTimeout)
	}
	if srv.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want 10s", srv.ReadTimeout)
	}
	if srv.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want 10s", srv.WriteTimeout)
	}
	if srv.IdleTimeout != 30*time.Second {
		t.Errorf("IdleTimeout = %v, want 30s", srv.IdleTimeout)
	}
}

func TestNewPreservesHandler(t *testing.T) {
	cfg := validTestConfig()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test-Handler", "preserved")
		w.WriteHeader(http.StatusAccepted)
	})

	srv, err := New(cfg, handler)
	if err != nil {
		t.Fatalf("New() unexpected error: %v", err)
	}

	req, err := http.NewRequest(http.MethodGet, "/test", nil)
	if err != nil {
		t.Fatal(err)
	}

	recorder := &responseRecorder{
		header: make(http.Header),
	}

	srv.Handler.ServeHTTP(recorder, req)

	if recorder.status != http.StatusAccepted {
		t.Errorf("status = %d, want 202", recorder.status)
	}
	if recorder.header.Get("X-Test-Handler") != "preserved" {
		t.Error("original handler was not preserved")
	}
}

type responseRecorder struct {
	header http.Header
	status int
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return len(data), nil
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
}

func TestNewRejectsNilHandler(t *testing.T) {
	cfg := validTestConfig()

	if _, err := New(cfg, nil); err == nil {
		t.Fatal("New() accepted nil handler")
	}
}

func TestNewRejectsInvalidHost(t *testing.T) {
	tests := []string{
		"",
		"0.0.0.0",
		"localhost",
		"192.168.1.10",
		"::1",
	}

	for _, host := range tests {
		t.Run(host, func(t *testing.T) {
			cfg := validTestConfig()
			cfg.Host = host

			if _, err := New(cfg, http.NotFoundHandler()); err == nil {
				t.Fatalf("New() accepted invalid host %q", host)
			}
		})
	}
}

func TestNewRejectsInvalidPort(t *testing.T) {
	tests := []int{-1, 0, 80, 1023, 65536, 99999}

	for _, port := range tests {
		t.Run("port", func(t *testing.T) {
			cfg := validTestConfig()
			cfg.Port = port

			if _, err := New(cfg, http.NotFoundHandler()); err == nil {
				t.Fatalf("New() accepted invalid port %d", port)
			}
		})
	}
}

func TestNewAcceptsPortBoundaries(t *testing.T) {
	for _, port := range []int{1024, 65535} {
		cfg := validTestConfig()
		cfg.Port = port

		if _, err := New(cfg, http.NotFoundHandler()); err != nil {
			t.Errorf("New() rejected valid port %d: %v", port, err)
		}
	}
}
