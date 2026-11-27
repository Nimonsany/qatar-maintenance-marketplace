package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHealthEndpoint(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)

	recorder := httptest.NewRecorder()
	newHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("unexpected content type: %q", got)
	}

	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("unexpected cache control: %q", got)
	}

	var response healthResponse

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("unexpected status: %q", response.Status)
	}

	if response.Service != "qatar-maintenance-api" {
		t.Errorf("unexpected service: %q", response.Service)
	}
}

func TestHealthRejectsPost(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/health",
		nil,
	)

	recorder := httptest.NewRecorder()
	newHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", recorder.Code)
	}
}

func TestUnknownRoute(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodGet,
		"/unknown",
		nil,
	)

	recorder := httptest.NewRecorder()
	newHandler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

func TestLoadPort(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      int
		wantError bool
	}{
		{"default", "", 8080, false},
		{"custom", "9090", 9090, false},
		{"minimum", "1024", 1024, false},
		{"maximum", "65535", 65535, false},
		{"non-numeric", "abc", 0, true},
		{"too-small", "80", 0, true},
		{"too-large", "65536", 0, true},
		{"negative", "-1", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadPort(tt.input)

			if (err != nil) != tt.wantError {
				t.Fatalf("unexpected error: %v", err)
			}

			if !tt.wantError && got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestServerConfiguration(t *testing.T) {
	server := newServer(9090)

	if server.Addr != "127.0.0.1:9090" {
		t.Errorf("unexpected address: %s", server.Addr)
	}

	if server.ReadHeaderTimeout != 5*time.Second {
		t.Error("incorrect read header timeout")
	}

	if server.ReadTimeout != 10*time.Second {
		t.Error("incorrect read timeout")
	}

	if server.WriteTimeout != 10*time.Second {
		t.Error("incorrect write timeout")
	}

	if server.IdleTimeout != 30*time.Second {
		t.Error("incorrect idle timeout")
	}
}
