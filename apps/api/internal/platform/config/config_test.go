package config

import (
	"log/slog"
	"strings"
	"testing"
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
	if cfg.LogLevel != slog.LevelDebug || cfg.TracesExporter != "none" {
		t.Fatalf("unexpected cfg %+v", cfg)
	}
}
