package config

import (
	"strings"
	"testing"
	"time"
)

// lookupFromMap simulates environment variables without
// modifying the actual system environment.
func lookupFromMap(values map[string]string) LookupEnv {
	return func(key string) (string, bool) {
		value, exists := values[key]
		return value, exists
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(lookupFromMap(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "127.0.0.1" {
		t.Errorf("unexpected host: %q", cfg.Host)
	}

	if cfg.Port != 8080 {
		t.Errorf("unexpected port: %d", cfg.Port)
	}

	if cfg.LogLevel != "INFO" {
		t.Errorf("unexpected log level: %q", cfg.LogLevel)
	}

	if cfg.ShutdownTimeout != 5*time.Second {
		t.Errorf("unexpected timeout: %s", cfg.ShutdownTimeout)
	}

	if cfg.Address() != "127.0.0.1:8080" {
		t.Errorf("unexpected address: %q", cfg.Address())
	}
}

func TestLoadValidCustomConfiguration(t *testing.T) {
	cfg, err := Load(lookupFromMap(map[string]string{
		"QMM_HOST":             "127.0.0.1",
		"QMM_PORT":             "9090",
		"QMM_LOG_LEVEL":        "debug",
		"QMM_SHUTDOWN_TIMEOUT": "10s",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Address() != "127.0.0.1:9090" {
		t.Errorf("unexpected address: %q", cfg.Address())
	}

	if cfg.LogLevel != "DEBUG" {
		t.Errorf("unexpected log level: %q", cfg.LogLevel)
	}

	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("unexpected timeout: %s", cfg.ShutdownTimeout)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]string
		errorKey string
	}{
		{
			name:     "public network host",
			values:   map[string]string{"QMM_HOST": "0.0.0.0"},
			errorKey: "QMM_HOST",
		},
		{
			name:     "external IP",
			values:   map[string]string{"QMM_HOST": "192.168.1.10"},
			errorKey: "QMM_HOST",
		},
		{
			name:     "invalid port",
			values:   map[string]string{"QMM_PORT": "abc"},
			errorKey: "QMM_PORT",
		},
		{
			name:     "port too low",
			values:   map[string]string{"QMM_PORT": "80"},
			errorKey: "QMM_PORT",
		},
		{
			name:     "port too high",
			values:   map[string]string{"QMM_PORT": "65536"},
			errorKey: "QMM_PORT",
		},
		{
			name:     "invalid log level",
			values:   map[string]string{"QMM_LOG_LEVEL": "TRACE"},
			errorKey: "QMM_LOG_LEVEL",
		},
		{
			name:     "zero shutdown timeout",
			values:   map[string]string{"QMM_SHUTDOWN_TIMEOUT": "0s"},
			errorKey: "QMM_SHUTDOWN_TIMEOUT",
		},
		{
			name:     "negative shutdown timeout",
			values:   map[string]string{"QMM_SHUTDOWN_TIMEOUT": "-1s"},
			errorKey: "QMM_SHUTDOWN_TIMEOUT",
		},
		{
			name:     "shutdown timeout too large",
			values:   map[string]string{"QMM_SHUTDOWN_TIMEOUT": "31s"},
			errorKey: "QMM_SHUTDOWN_TIMEOUT",
		},
		{
			name:     "invalid duration",
			values:   map[string]string{"QMM_SHUTDOWN_TIMEOUT": "tomorrow"},
			errorKey: "QMM_SHUTDOWN_TIMEOUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(lookupFromMap(tt.values))

			if err == nil {
				t.Fatal("expected configuration error")
			}

			if !strings.Contains(err.Error(), tt.errorKey) {
				t.Errorf(
					"expected error mentioning %s, got %q",
					tt.errorKey,
					err.Error(),
				)
			}
		})
	}
}

func TestLoadEmptyPortPreservesDefault(t *testing.T) {
	cfg, err := Load(lookupFromMap(map[string]string{
		"QMM_PORT": "",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Port)
	}
}

func TestLoadRejectsNilLookup(t *testing.T) {
	_, err := Load(nil)
	if err == nil {
		t.Fatal("expected error for nil lookup")
	}
}

func TestLoadPortBoundaries(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"1024", 1024},
		{"65535", 65535},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cfg, err := Load(lookupFromMap(map[string]string{
				"QMM_PORT": tt.input,
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.Port != tt.want {
				t.Errorf("expected %d, got %d", tt.want, cfg.Port)
			}
		})
	}
}

func TestLoadShutdownTimeoutBoundaries(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"1ms", time.Millisecond},
		{"30s", 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			cfg, err := Load(lookupFromMap(map[string]string{
				"QMM_SHUTDOWN_TIMEOUT": tt.input,
			}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.ShutdownTimeout != tt.want {
				t.Errorf(
					"expected %s, got %s",
					tt.want,
					cfg.ShutdownTimeout,
				)
			}
		})
	}
}
