// Package config loads process configuration from the environment.
// It fails fast: a missing required variable is a startup error, never a silent default.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/telemetry"
)

// Config is the complete runtime configuration of the API process.
type Config struct {
	HTTPAddr    string
	DatabaseURL string
	LogLevel    slog.Level
	AuthMode    string // validated by auth.New; only "dev-stub" exists today
	// One exporter per OTel signal, under the OTel SDK's variable names (ADR 0021): OTEL_TRACES_EXPORTER
	// (none|stdout|otlp), OTEL_METRICS_EXPORTER and OTEL_LOGS_EXPORTER (none|otlp), none by default; and
	// OTEL_EXPORTER_OTLP_ENDPOINT, the OTLP/HTTP base URL, required when any of them is otlp.
	Telemetry telemetry.Config
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
	exporter := func(key string, allowed ...telemetry.Exporter) telemetry.Exporter {
		v := telemetry.Exporter(strings.TrimSpace(getenv(key)))
		if v == "" {
			return telemetry.ExporterNone
		}
		if !slices.Contains(allowed, v) {
			errs = append(errs, fmt.Errorf("invalid %s %q: want one of %v", key, v, allowed))
		}
		return v
	}

	cfg := Config{
		HTTPAddr:    required("HTTP_ADDR"),
		DatabaseURL: required("DATABASE_URL"),
		AuthMode:    required("AUTH_MODE"),
		Telemetry: telemetry.Config{
			Traces: exporter("OTEL_TRACES_EXPORTER",
				telemetry.ExporterNone, telemetry.ExporterStdout, telemetry.ExporterOTLP),
			Metrics: exporter("OTEL_METRICS_EXPORTER", telemetry.ExporterNone, telemetry.ExporterOTLP),
			Logs:    exporter("OTEL_LOGS_EXPORTER", telemetry.ExporterNone, telemetry.ExporterOTLP),
		},
	}

	if v := getenv("LOG_LEVEL"); v != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			errs = append(errs, fmt.Errorf("invalid LOG_LEVEL %q: %w", v, err))
		}
	}
	t := &cfg.Telemetry
	if slices.Contains([]telemetry.Exporter{t.Traces, t.Metrics, t.Logs}, telemetry.ExporterOTLP) {
		u, err := otlpEndpoint(strings.TrimSpace(getenv("OTEL_EXPORTER_OTLP_ENDPOINT")))
		if err != nil {
			errs = append(errs, err)
		}
		t.Endpoint = u
	}

	return cfg, errors.Join(errs...)
}

// otlpEndpoint parses the OTLP/HTTP base URL; telemetry appends the per-signal path (/v1/traces, ...).
func otlpEndpoint(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, errors.New("missing required env var OTEL_EXPORTER_OTLP_ENDPOINT (an OTEL_*_EXPORTER is otlp)")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("invalid OTEL_EXPORTER_OTLP_ENDPOINT %q: want http(s)://host:port", raw)
	}
	return u, nil
}
