// Package config loads process configuration from the environment.
// It fails fast: a missing required variable is a startup error, never a silent default.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// Config is the complete runtime configuration of the API process.
type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	LogLevel       slog.Level
	TracesExporter string // "none" (default) or "stdout"
}

// Load reads configuration via getenv (os.Getenv in production, a map in tests).
func Load(getenv func(string) string) (Config, error) {
	var errs []error
	required := func(key string) string {
		v := strings.TrimSpace(getenv(key))
		if v == "" {
			errs = append(errs, fmt.Errorf("missing required env var %s", key))
		}
		return v
	}

	cfg := Config{
		HTTPAddr:       required("HTTP_ADDR"),
		DatabaseURL:    required("DATABASE_URL"),
		TracesExporter: "none",
	}

	if v := getenv("LOG_LEVEL"); v != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			errs = append(errs, fmt.Errorf("invalid LOG_LEVEL %q: %w", v, err))
		}
	}
	if v := getenv("OTEL_TRACES_EXPORTER"); v != "" {
		if v != "none" && v != "stdout" {
			errs = append(errs, fmt.Errorf("invalid OTEL_TRACES_EXPORTER %q: want none|stdout", v))
		}
		cfg.TracesExporter = v
	}

	return cfg, errors.Join(errs...)
}
