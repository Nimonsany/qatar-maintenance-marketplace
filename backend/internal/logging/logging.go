package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// New creates a JSON logger with a validated minimum log level.
// Callers must not include credentials or raw request bodies in messages.
func New(output io.Writer, level string) (*slog.Logger, error) {
	if output == nil {
		return nil, fmt.Errorf("logging output is required")
	}

	var minLevel slog.Level

	switch strings.ToUpper(level) {
	case "DEBUG":
		minLevel = slog.LevelDebug
	case "INFO":
		minLevel = slog.LevelInfo
	case "WARN":
		minLevel = slog.LevelWarn
	case "ERROR":
		minLevel = slog.LevelError
	default:
		return nil, fmt.Errorf("invalid log level: expected DEBUG, INFO, WARN or ERROR")
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		Level: minLevel,
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if isSensitiveKey(attr.Key) {
				return slog.String(attr.Key, "[REDACTED]")
			}
			return attr
		},
	})

	return slog.New(handler), nil
}

func isSensitiveKey(key string) bool {
	normalized := strings.NewReplacer(
		"_", "",
		"-", "",
		".", "",
		" ", "",
	).Replace(strings.ToLower(key))

	sensitive := []string{
		"password",
		"passwd",
		"secret",
		"token",
		"authorization",
		"cookie",
		"apikey",
		"credential",
		"session",
		"otp",
	}

	for _, word := range sensitive {
		if strings.Contains(normalized, word) {
			return true
		}
	}

	return false
}
