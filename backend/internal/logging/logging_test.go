package logging

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func newTestLogger(t *testing.T, level string) (*bytes.Buffer, func(string, ...any)) {
	t.Helper()

	var output bytes.Buffer

	logger, err := New(&output, level)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	return &output, func(message string, args ...any) {
		logger.Info(message, args...)
	}
}

func parseLogLine(t *testing.T, output string) map[string]any {
	t.Helper()

	var entry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &entry); err != nil {
		t.Fatalf("invalid JSON log: %v", err)
	}

	return entry
}

func TestNewJSONOutput(t *testing.T) {
	output, logInfo := newTestLogger(t, "INFO")

	logInfo("request completed", "request_id", "req-123", "status", 200)

	entry := parseLogLine(t, output.String())

	if entry["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", entry["level"])
	}
	if entry["msg"] != "request completed" {
		t.Errorf("msg = %v, want request completed", entry["msg"])
	}
	if entry["request_id"] != "req-123" {
		t.Errorf("request_id = %v, want req-123", entry["request_id"])
	}
	if entry["status"] != float64(200) {
		t.Errorf("status = %v, want 200", entry["status"])
	}
	if _, ok := entry["time"]; !ok {
		t.Error("missing time field")
	}
}

func TestNewRejectsInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name  string
		level string
	}{
		{"empty", ""},
		{"invalid", "TRACE"},
		{"numeric", "5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(&bytes.Buffer{}, tt.level); err == nil {
				t.Fatalf("New() accepted invalid level %q", tt.level)
			}
		})
	}

	if _, err := New(nil, "INFO"); err == nil {
		t.Error("New() accepted nil output")
	}
}

func TestNewAcceptsAllLevels(t *testing.T) {
	for _, level := range []string{
		"DEBUG", "INFO", "WARN", "ERROR", "debug", "info",
	} {
		t.Run(level, func(t *testing.T) {
			if _, err := New(&bytes.Buffer{}, level); err != nil {
				t.Fatalf("New(%q) error: %v", level, err)
			}
		})
	}
}

func TestLogLevelFiltering(t *testing.T) {
	var output bytes.Buffer

	logger, err := New(&output, "WARN")
	if err != nil {
		t.Fatal(err)
	}

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warning message")
	logger.Error("error message")

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("log lines = %d, want 2", len(lines))
	}

	first := parseLogLine(t, lines[0])
	second := parseLogLine(t, lines[1])

	if first["level"] != "WARN" || second["level"] != "ERROR" {
		t.Errorf("unexpected levels: %v, %v", first["level"], second["level"])
	}
}

func TestSensitiveAttributesRedacted(t *testing.T) {
	var output bytes.Buffer

	logger, err := New(&output, "INFO")
	if err != nil {
		t.Fatal(err)
	}

	logger.Info(
		"security audit",
		"password", "password-value-123",
		"access_token", "token-value-456",
		"api_key", "api-value-789",
		"Authorization", "bearer-value-abc",
		"session_id", "session-value-def",
		"request_id", "safe-request-123",
	)

	entry := parseLogLine(t, output.String())

	for _, key := range []string{
		"password",
		"access_token",
		"api_key",
		"Authorization",
		"session_id",
	} {
		if entry[key] != "[REDACTED]" {
			t.Errorf("%s = %v, want [REDACTED]", key, entry[key])
		}
	}

	if entry["request_id"] != "safe-request-123" {
		t.Errorf("request_id should remain visible, got %v", entry["request_id"])
	}

	for _, secret := range []string{
		"password-value-123",
		"token-value-456",
		"api-value-789",
		"bearer-value-abc",
		"session-value-def",
	} {
		if strings.Contains(output.String(), secret) {
			t.Errorf("sensitive value appeared in log output")
		}
	}
}

func TestSensitiveGroupedAttributesRedacted(t *testing.T) {
	var output bytes.Buffer

	logger, err := New(&output, "INFO")
	if err != nil {
		t.Fatal(err)
	}

	logger.Info(
		"nested attributes",
		slog.Group(
			"user",
			"password", "nested-secret-123",
			"role", "customer",
		),
	)

	if strings.Contains(output.String(), "nested-secret-123") {
		t.Fatal("nested secret appeared in log output")
	}

	entry := parseLogLine(t, output.String())
	user, ok := entry["user"].(map[string]any)
	if !ok {
		t.Fatalf("user group missing: %v", entry["user"])
	}

	if user["password"] != "[REDACTED]" {
		t.Errorf("nested password not redacted: %v", user["password"])
	}
	if user["role"] != "customer" {
		t.Errorf("safe nested attribute changed: %v", user["role"])
	}
}
