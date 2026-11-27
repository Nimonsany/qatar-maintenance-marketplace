package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHost            = "127.0.0.1"
	DefaultPort            = 8080
	DefaultLogLevel        = "INFO"
	DefaultShutdownTimeout = 5 * time.Second
)

// Config contains validated application configuration.
type Config struct {
	Host            string
	Port            int
	LogLevel        string
	ShutdownTimeout time.Duration
}

// LookupEnv allows configuration to be tested without
// changing process-wide environment variables.
//
// The bool indicates whether an environment variable exists.
type LookupEnv func(key string) (string, bool)

// Load reads and validates application configuration.
//
// Only safe local development settings are supported.
// No database, payment or authentication configuration
// is added in DEV-004A.
func Load(lookup LookupEnv) (Config, error) {
	cfg := Config{
		Host:            DefaultHost,
		Port:            DefaultPort,
		LogLevel:        DefaultLogLevel,
		ShutdownTimeout: DefaultShutdownTimeout,
	}

	if lookup == nil {
		return Config{}, fmt.Errorf("configuration lookup is required")
	}

	if raw, exists := lookup("QMM_HOST"); exists {
		if raw != DefaultHost {
			return Config{}, fmt.Errorf(
				"invalid QMM_HOST: only 127.0.0.1 is permitted",
			)
		}
		cfg.Host = raw
	}

	if raw, exists := lookup("QMM_PORT"); exists {
		// Preserve DEV-002 behavior: an empty value uses
		// the default port.
		if raw != "" {
			port, err := strconv.Atoi(raw)
			if err != nil || port < 1024 || port > 65535 {
				return Config{}, fmt.Errorf(
					"invalid QMM_PORT: expected integer from 1024 to 65535",
				)
			}
			cfg.Port = port
		}
	}

	if raw, exists := lookup("QMM_LOG_LEVEL"); exists {
		level := strings.ToUpper(raw)

		switch level {
		case "DEBUG", "INFO", "WARN", "ERROR":
			cfg.LogLevel = level
		default:
			return Config{}, fmt.Errorf(
				"invalid QMM_LOG_LEVEL: expected DEBUG, INFO, WARN or ERROR",
			)
		}
	}

	if raw, exists := lookup("QMM_SHUTDOWN_TIMEOUT"); exists {
		duration, err := time.ParseDuration(raw)
		if err != nil || duration <= 0 || duration > 30*time.Second {
			return Config{}, fmt.Errorf(
				"invalid QMM_SHUTDOWN_TIMEOUT: expected positive duration up to 30s",
			)
		}
		cfg.ShutdownTimeout = duration
	}

	return cfg, nil
}

// Address constructs the validated localhost listening address.
func (cfg Config) Address() string {
	return net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
}
