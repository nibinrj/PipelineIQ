package config

import (
	"strings"
	"testing"
	"time"
)

func TestFromEnv(t *testing.T) {
	t.Setenv("PIPELINEIQ_DATABASE_URL", "")
	t.Setenv("PIPELINEIQ_HTTP_ADDR", "")
	t.Setenv("PIPELINEIQ_SHUTDOWN_TIMEOUT", "")
	t.Setenv("PIPELINEIQ_READY_TIMEOUT", "")
	t.Setenv("PIPELINEIQ_LOG_LEVEL", "")
	t.Setenv("PIPELINEIQ_INGEST_KEY", "")

	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
		check   func(t *testing.T, cfg Config)
	}{
		{
			name:    "database url required",
			wantErr: "PIPELINEIQ_DATABASE_URL is required",
		},
		{
			name:    "ingest key required",
			env:     map[string]string{"PIPELINEIQ_DATABASE_URL": "postgres://example"},
			wantErr: "PIPELINEIQ_INGEST_KEY is required",
		},
		{
			name: "defaults",
			env: map[string]string{
				"PIPELINEIQ_DATABASE_URL": "postgres://example",
				"PIPELINEIQ_INGEST_KEY":   "local-key",
			},
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.HTTPAddr != ":8080" {
					t.Errorf("HTTPAddr = %q", cfg.HTTPAddr)
				}
				if cfg.ShutdownTimeout != 10*time.Second {
					t.Errorf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
				}
				if cfg.ReadyTimeout != 2*time.Second {
					t.Errorf("ReadyTimeout = %s", cfg.ReadyTimeout)
				}
				if cfg.LogLevel != "info" {
					t.Errorf("LogLevel = %q", cfg.LogLevel)
				}
			},
		},
		{
			name: "overrides",
			env: map[string]string{
				"PIPELINEIQ_DATABASE_URL":     "postgres://example",
				"PIPELINEIQ_INGEST_KEY":       "local-key",
				"PIPELINEIQ_HTTP_ADDR":        "127.0.0.1:9090",
				"PIPELINEIQ_SHUTDOWN_TIMEOUT": "3s",
				"PIPELINEIQ_READY_TIMEOUT":    "500ms",
				"PIPELINEIQ_LOG_LEVEL":        "WARN",
			},
			check: func(t *testing.T, cfg Config) {
				t.Helper()
				if cfg.HTTPAddr != "127.0.0.1:9090" || cfg.ShutdownTimeout != 3*time.Second || cfg.ReadyTimeout != 500*time.Millisecond || cfg.LogLevel != "warn" {
					t.Errorf("cfg = %+v", cfg)
				}
			},
		},
		{
			name: "bad duration",
			env: map[string]string{
				"PIPELINEIQ_DATABASE_URL":     "postgres://example",
				"PIPELINEIQ_SHUTDOWN_TIMEOUT": "soon",
			},
			wantErr: "PIPELINEIQ_SHUTDOWN_TIMEOUT",
		},
		{
			name: "non positive duration",
			env: map[string]string{
				"PIPELINEIQ_DATABASE_URL":  "postgres://example",
				"PIPELINEIQ_READY_TIMEOUT": "0s",
			},
			wantErr: "PIPELINEIQ_READY_TIMEOUT must be greater than 0",
		},
		{
			name: "bad log level",
			env: map[string]string{
				"PIPELINEIQ_DATABASE_URL": "postgres://example",
				"PIPELINEIQ_LOG_LEVEL":    "loud",
			},
			wantErr: "PIPELINEIQ_LOG_LEVEL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg, err := FromEnv()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromEnv: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}
