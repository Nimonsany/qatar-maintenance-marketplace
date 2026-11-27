package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"qatar-maintenance/backend/internal/config"
	"qatar-maintenance/backend/internal/logging"
	serverpkg "qatar-maintenance/backend/internal/server"
	"time"
)

func performRequest(method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, nil)
	recorder := httptest.NewRecorder()

	newHandler().ServeHTTP(recorder, request)
	return recorder
}

func decodeAPIError(t *testing.T, recorder *httptest.ResponseRecorder) APIErrorResponse {
	t.Helper()

	var response APIErrorResponse

	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON error: %v", err)
	}

	return response
}

func TestHealthEndpoint(t *testing.T) {
	recorder := performRequest(http.MethodGet, "/health")

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
	recorder := performRequest(http.MethodPost, "/health")

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", recorder.Code)
	}

	if got := recorder.Header().Get("Allow"); got != "GET" {
		t.Errorf("expected Allow GET, got %q", got)
	}

	response := decodeAPIError(t, recorder)

	if response.Error.Code != ErrorCodeMethodNotAllowed {
		t.Errorf("unexpected error code: %q", response.Error.Code)
	}

	if response.Error.Message != "HTTP method not allowed" {
		t.Errorf("unexpected message: %q", response.Error.Message)
	}

	if response.Error.RequestID == "" {
		t.Error("missing request ID in error response")
	}
}

func TestUnknownRoute(t *testing.T) {
	recorder := performRequest(http.MethodGet, "/unknown")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}

	response := decodeAPIError(t, recorder)

	if response.Error.Code != ErrorCodeNotFound {
		t.Errorf("unexpected error code: %q", response.Error.Code)
	}

	if response.Error.Message != "Resource not found" {
		t.Errorf("unexpected message: %q", response.Error.Message)
	}
}

func TestHeadHealthRejectsWithoutBody(t *testing.T) {
	recorder := performRequest(http.MethodHead, "/health")

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", recorder.Code)
	}

	if recorder.Body.Len() != 0 {
		t.Error("HEAD response must not have a body")
	}

	if got := recorder.Header().Get("Allow"); got != "GET" {
		t.Errorf("expected Allow GET, got %q", got)
	}

	if recorder.Header().Get("X-Request-ID") == "" {
		t.Error("missing request ID")
	}
}

func TestHeadUnknownRouteWithoutBody(t *testing.T) {
	recorder := performRequest(http.MethodHead, "/unknown")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}

	if recorder.Body.Len() != 0 {
		t.Error("HEAD response must not have a body")
	}
}

func TestIntegrationSecurityHeaders(t *testing.T) {
	paths := []string{"/health", "/unknown"}

	expected := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			recorder := performRequest(http.MethodGet, path)

			for name, value := range expected {
				if got := recorder.Header().Get(name); got != value {
					t.Errorf(
						"%s: expected %q, got %q",
						name,
						value,
						got,
					)
				}
			}

			if got := recorder.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("unexpected Content-Type: %q", got)
			}
		})
	}
}

func TestIntegrationRequestIDs(t *testing.T) {
	first := performRequest(http.MethodGet, "/health")
	second := performRequest(http.MethodGet, "/health")

	id1 := first.Header().Get("X-Request-ID")
	id2 := second.Header().Get("X-Request-ID")

	pattern := regexp.MustCompile(`^[0-9a-f]{32}$`)

	if !pattern.MatchString(id1) {
		t.Errorf("invalid first ID: %q", id1)
	}

	if !pattern.MatchString(id2) {
		t.Errorf("invalid second ID: %q", id2)
	}

	if id1 == id2 {
		t.Error("independent requests have identical IDs")
	}
}

func TestIntegrationErrorIDMatchesHeader(t *testing.T) {
	recorder := performRequest(http.MethodGet, "/missing")

	response := decodeAPIError(t, recorder)

	headerID := recorder.Header().Get("X-Request-ID")

	if headerID == "" {
		t.Fatal("missing X-Request-ID header")
	}

	if response.Error.RequestID != headerID {
		t.Errorf(
			"body ID %q does not match header ID %q",
			response.Error.RequestID,
			headerID,
		)
	}
}

func TestIntegrationIgnoresClientID(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	request.Header.Set("X-Request-ID", "client-controlled-id")

	recorder := httptest.NewRecorder()
	newHandler().ServeHTTP(recorder, request)

	if got := recorder.Header().Get("X-Request-ID"); got == "client-controlled-id" {
		t.Error("server accepted client-supplied request ID")
	}

	if recorder.Header().Get("X-Request-ID") == "" {
		t.Error("server did not generate a request ID")
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

func TestIntegratedApplicationComponents(t *testing.T) {
	values := map[string]string{
		"QMM_HOST":             "127.0.0.1",
		"QMM_PORT":             "9090",
		"QMM_LOG_LEVEL":        "debug",
		"QMM_SHUTDOWN_TIMEOUT": "10s",
	}

	lookup := func(key string) (string, bool) {
		value, exists := values[key]
		return value, exists
	}

	cfg, err := config.Load(lookup)
	if err != nil {
		t.Fatalf("config.Load() failed: %v", err)
	}

	if cfg.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Port)
	}

	if cfg.LogLevel != "DEBUG" {
		t.Errorf("log level = %q, want DEBUG", cfg.LogLevel)
	}

	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("shutdown timeout = %v, want 10s", cfg.ShutdownTimeout)
	}

	var output bytes.Buffer

	logger, err := logging.New(&output, cfg.LogLevel)
	if err != nil {
		t.Fatalf("logging.New() failed: %v", err)
	}

	srv, err := serverpkg.New(cfg, newHandler())
	if err != nil {
		t.Fatalf("server.New() failed: %v", err)
	}

	if srv.Addr != "127.0.0.1:9090" {
		t.Errorf("address = %q, want 127.0.0.1:9090", srv.Addr)
	}

	logger.Info(
		"application starting",
		"service", "qatar-maintenance-api",
		"address", srv.Addr,
	)

	var entry map[string]any

	if err := json.Unmarshal(
		bytes.TrimSpace(output.Bytes()), &entry,
	); err != nil {
		t.Fatalf("startup log is not valid JSON: %v", err)
	}

	if entry["level"] != "INFO" {
		t.Errorf("log level = %v, want INFO", entry["level"])
	}

	if entry["msg"] != "application starting" {
		t.Errorf("unexpected log message: %v", entry["msg"])
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()

	srv.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Errorf("unexpected health response: %s", recorder.Body.String())
	}

	if recorder.Header().Get("X-Request-ID") == "" {
		t.Error("missing server-generated request ID")
	}

	if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("security headers were not preserved")
	}
}

func TestIntegratedConfigurationRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"public host", "QMM_HOST", "0.0.0.0"},
		{"invalid port", "QMM_PORT", "80"},
		{"invalid log level", "QMM_LOG_LEVEL", "TRACE"},
		{"invalid shutdown timeout", "QMM_SHUTDOWN_TIMEOUT", "31s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lookup := func(key string) (string, bool) {
				if key == tt.key {
					return tt.value, true
				}
				return "", false
			}

			if _, err := config.Load(lookup); err == nil {
				t.Errorf(
					"configuration accepted %s=%q",
					tt.key,
					tt.value,
				)
			}
		})
	}
}
