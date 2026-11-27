package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"qatar-maintenance/backend/internal/config"
	"qatar-maintenance/backend/internal/logging"
	serverpkg "qatar-maintenance/backend/internal/server"
	"syscall"
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

func TestServerConfiguration(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "QMM_PORT" {
			return "9090", true
		}
		return "", false
	}

	var logs bytes.Buffer

	_, srv, _, err := buildApplication(lookup, &logs)
	if err != nil {
		t.Fatalf("buildApplication() failed: %v", err)
	}

	if srv == nil {
		t.Fatal("expected non-nil HTTP server")
	}

	if srv.Addr != "127.0.0.1:9090" {
		t.Errorf(
			"server address = %q, want 127.0.0.1:9090",
			srv.Addr,
		)
	}

	if srv.Handler == nil {
		t.Error("HTTP handler must not be nil")
	}

	if srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("incorrect read header timeout: %v", srv.ReadHeaderTimeout)
	}

	if srv.ReadTimeout != 10*time.Second {
		t.Errorf("incorrect read timeout: %v", srv.ReadTimeout)
	}

	if srv.WriteTimeout != 10*time.Second {
		t.Errorf("incorrect write timeout: %v", srv.WriteTimeout)
	}

	if srv.IdleTimeout != 30*time.Second {
		t.Errorf("incorrect idle timeout: %v", srv.IdleTimeout)
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

func TestBuildApplicationSuccess(t *testing.T) {
	values := map[string]string{
		"QMM_HOST":             "127.0.0.1",
		"QMM_PORT":             "9090",
		"QMM_LOG_LEVEL":        "DEBUG",
		"QMM_SHUTDOWN_TIMEOUT": "10s",
	}

	lookup := func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}

	var logs bytes.Buffer

	cfg, srv, logger, err := buildApplication(
		lookup,
		&logs,
	)
	if err != nil {
		t.Fatalf("buildApplication() failed: %v", err)
	}

	if srv == nil {
		t.Fatal("expected HTTP server")
	}

	if logger == nil {
		t.Fatal("expected logger")
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("host = %q, want 127.0.0.1", cfg.Host)
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

	if srv.Addr != "127.0.0.1:9090" {
		t.Errorf("server address = %q, want 127.0.0.1:9090", srv.Addr)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want 200", rec.Code)
	}

	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("missing server-generated request ID")
	}

	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("security header missing")
	}

	logger.Info("test startup")

	var logEntry map[string]any
	if err := json.Unmarshal(
		bytes.TrimSpace(logs.Bytes()),
		&logEntry,
	); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}

	if logEntry["msg"] != "test startup" {
		t.Errorf("unexpected log message: %v", logEntry["msg"])
	}
}

func TestBuildApplicationInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"invalid host", "QMM_HOST", "0.0.0.0"},
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

			var logs bytes.Buffer

			_, srv, logger, err := buildApplication(
				lookup,
				&logs,
			)

			if err == nil {
				t.Fatal("expected configuration error")
			}

			if srv != nil {
				t.Error("server should not be created")
			}

			if logger != nil {
				t.Error("logger should not be created")
			}
		})
	}
}

func TestBuildApplicationRejectsNilDependencies(t *testing.T) {
	t.Run("nil lookup", func(t *testing.T) {
		var logs bytes.Buffer

		_, srv, logger, err := buildApplication(
			nil,
			&logs,
		)

		if err == nil {
			t.Fatal("expected error for nil lookup")
		}

		if srv != nil || logger != nil {
			t.Fatal("unexpected initialized components")
		}
	})

	t.Run("nil log output", func(t *testing.T) {
		lookup := func(string) (string, bool) {
			return "", false
		}

		_, srv, logger, err := buildApplication(
			lookup,
			nil,
		)

		if err == nil {
			t.Fatal("expected error for nil log output")
		}

		if srv != nil || logger != nil {
			t.Fatal("unexpected initialized components")
		}
	})
}

func TestApplicationOccupiedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("localhost listener unavailable: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("find test executable: %v", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		executable,
		"-test.run=^TestApplicationSubprocessHelper$",
	)

	// Do not inherit conflicting QMM_* values from the parent.
	var env []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "QMM_") {
			continue
		}
		env = append(env, entry)
	}

	cmd.Env = append(
		env,
		"QMM_TEST_CHILD=1",
		"QMM_HOST=127.0.0.1",
		fmt.Sprintf("QMM_PORT=%d", port),
		"QMM_LOG_LEVEL=INFO",
		"QMM_SHUTDOWN_TIMEOUT=5s",
	)

	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf(
			"application did not exit after port conflict:\n%s",
			output,
		)
	}

	if err == nil {
		t.Fatalf(
			"expected non-zero exit on occupied port:\n%s",
			output,
		)
	}

	if !bytes.Contains(
		output,
		[]byte("address already in use"),
	) {
		t.Fatalf(
			"expected address-in-use error, got:\n%s",
			output,
		)
	}

	if !bytes.Contains(
		output,
		[]byte(`"level":"ERROR"`),
	) {
		t.Fatalf(
			"expected JSON error log, got:\n%s",
			output,
		)
	}
}

func TestApplicationSubprocessHelper(t *testing.T) {
	if os.Getenv("QMM_TEST_CHILD") != "1" {
		return
	}

	main()
}

func TestApplicationSIGTERMGracefulShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("localhost listener unavailable: %v", err)
	}

	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("release test port: %v", err)
	}

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("find test executable: %v", err)
	}

	cmd := exec.Command(
		executable,
		"-test.run=^TestApplicationSubprocessHelper$",
	)

	// Remove inherited QMM_* variables so the child has
	// exactly the intended test configuration.
	var env []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "QMM_") {
			continue
		}
		env = append(env, entry)
	}

	cmd.Env = append(
		env,
		"QMM_TEST_CHILD=1",
		"QMM_HOST=127.0.0.1",
		fmt.Sprintf("QMM_PORT=%d", port),
		"QMM_LOG_LEVEL=INFO",
		"QMM_SHUTDOWN_TIMEOUT=5s",
	)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		t.Fatalf("start subprocess: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	waitConsumed := false

	defer func() {
		if waitConsumed {
			return
		}

		_ = cmd.Process.Kill()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("subprocess cleanup timed out")
		}
	}()

	client := &http.Client{
		Timeout: 300 * time.Millisecond,
	}
	url := fmt.Sprintf(
		"http://127.0.0.1:%d/health",
		port,
	)

	deadline := time.Now().Add(5 * time.Second)
	ready := false

	for time.Now().Before(deadline) {
		select {

		case err := <-done:
			waitConsumed = true
			t.Fatalf(
				"application exited before readiness: %v",
				err,
			)

		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
	}

	if !ready {
		t.Fatal("application did not become ready")
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("send SIGTERM: %v", err)
	}

	select {
	case err := <-done:
		waitConsumed = true
		if err != nil {
			t.Fatalf(
				"application exited with error: %v\n%s",
				err,
				output.String(),
			)
		}

	case <-time.After(8 * time.Second):
		t.Fatal("application did not shut down in time")
	}

	if !bytes.Contains(
		output.Bytes(),
		[]byte(`"msg":"application shutdown completed"`),
	) {
		t.Fatalf(
			"graceful shutdown log missing:\n%s",
			output.String(),
		)
	}

	if !bytes.Contains(
		output.Bytes(),
		[]byte(`"level":"INFO"`),
	) {
		t.Fatal("expected structured INFO log")
	}
}
