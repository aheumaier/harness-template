package config

import (
	"log/slog"
	"maps"
	"strings"
	"testing"

	"github.com/aheumaier/harness-template/apps/api/internal/platform/telemetry"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoad_FailsFastOnMissing(t *testing.T) {
	_, err := Load(env(nil))
	if err == nil || !strings.Contains(err.Error(), "HTTP_ADDR") || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("want error naming both missing vars, got %v", err)
	}
}

func TestLoad_OK(t *testing.T) {
	cfg, err := Load(env(map[string]string{"HTTP_ADDR": ":8080", "DATABASE_URL": "postgres://x", "AUTH_MODE": "dev-stub", "LOG_LEVEL": "debug"}))
	if err != nil {
		t.Fatal(err)
	}
	none := telemetry.Config{Traces: telemetry.ExporterNone, Metrics: telemetry.ExporterNone, Logs: telemetry.ExporterNone}
	if cfg.LogLevel != slog.LevelDebug || cfg.Telemetry != none {
		t.Fatalf("unexpected cfg %+v", cfg)
	}
}

func TestLoad_Exporters(t *testing.T) {
	base := map[string]string{"HTTP_ADDR": ":8080", "DATABASE_URL": "postgres://x", "AUTH_MODE": "dev-stub"}
	const endpoint = "http://127.0.0.1:4318"
	for name, tc := range map[string]struct {
		env     map[string]string
		wantErr string // "" = valid
	}{
		"otlp everywhere": {map[string]string{"OTEL_TRACES_EXPORTER": "otlp", "OTEL_METRICS_EXPORTER": "otlp",
			"OTEL_LOGS_EXPORTER": "otlp", "OTEL_EXPORTER_OTLP_ENDPOINT": endpoint}, ""},
		"stdout traces need no endpoint": {map[string]string{"OTEL_TRACES_EXPORTER": "stdout"}, ""},
		"metrics only otlp": {map[string]string{"OTEL_METRICS_EXPORTER": "otlp",
			"OTEL_EXPORTER_OTLP_ENDPOINT": endpoint}, ""},
		"logs only otlp, padded": {map[string]string{"OTEL_LOGS_EXPORTER": " otlp ",
			"OTEL_EXPORTER_OTLP_ENDPOINT": " " + endpoint + " "}, ""},
		"endpoint ignored when nothing is otlp": {map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "nonsense"}, ""},
		"otlp without endpoint": {map[string]string{"OTEL_LOGS_EXPORTER": "otlp"},
			"missing required env var OTEL_EXPORTER_OTLP_ENDPOINT"},
		"endpoint without scheme": {map[string]string{"OTEL_METRICS_EXPORTER": "otlp",
			"OTEL_EXPORTER_OTLP_ENDPOINT": "127.0.0.1:4318"}, "invalid OTEL_EXPORTER_OTLP_ENDPOINT"},
		"endpoint with query": {map[string]string{"OTEL_TRACES_EXPORTER": "otlp",
			"OTEL_EXPORTER_OTLP_ENDPOINT": "http://h:4318?x=1"}, "invalid OTEL_EXPORTER_OTLP_ENDPOINT"},
		"unknown traces exporter": {map[string]string{"OTEL_TRACES_EXPORTER": "jaeger"}, "invalid OTEL_TRACES_EXPORTER"},
		"stdout is traces only":   {map[string]string{"OTEL_METRICS_EXPORTER": "stdout"}, "invalid OTEL_METRICS_EXPORTER"},
		"console is not stdout":   {map[string]string{"OTEL_LOGS_EXPORTER": "console"}, "invalid OTEL_LOGS_EXPORTER"},
	} {
		t.Run(name, func(t *testing.T) {
			m := maps.Clone(base)
			maps.Copy(m, tc.env)
			cfg, err := Load(env(m))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				tel := cfg.Telemetry
				anyOTLP := tel.Traces == telemetry.ExporterOTLP || tel.Metrics == telemetry.ExporterOTLP ||
					tel.Logs == telemetry.ExporterOTLP
				if gotURL := tel.Endpoint != nil; gotURL != anyOTLP || (gotURL && tel.Endpoint.String() != endpoint) {
					t.Fatalf("endpoint %v for %+v", tel.Endpoint, tel)
				}
				for key, got := range map[string]telemetry.Exporter{
					"OTEL_TRACES_EXPORTER": tel.Traces, "OTEL_METRICS_EXPORTER": tel.Metrics, "OTEL_LOGS_EXPORTER": tel.Logs,
				} {
					if want := strings.TrimSpace(m[key]); got != telemetry.Exporter(want) && (want != "" || got != telemetry.ExporterNone) {
						t.Errorf("%s = %q, want %q", key, got, want)
					}
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
