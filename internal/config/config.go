// Package config reads process environment variables. There is no config file
// in P1; rule thresholds arrive as YAML in a later prompt.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nibinrj/PipelineIQ/internal/rules"
)

// Config is everything the process needs to start. The database URL is never
// logged: it contains the password.
type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
	ReadyTimeout    time.Duration
	LogLevel        string
	// IngestKey is the bearer token for POST /api/v1/builds. It is required so
	// an empty configuration cannot leave ingest open.
	IngestKey string
	// Rules are the flaky and quarantine thresholds. Defaults match the plan.
	Rules rules.Thresholds
}

// FromEnv reads PIPELINEIQ_* variables. Missing optional values get the
// defaults documented in .env.example.
func FromEnv() (Config, error) {
	cfg := Config{
		HTTPAddr:        envOr("PIPELINEIQ_HTTP_ADDR", ":8080"),
		DatabaseURL:     strings.TrimSpace(os.Getenv("PIPELINEIQ_DATABASE_URL")),
		ShutdownTimeout: 10 * time.Second,
		ReadyTimeout:    2 * time.Second,
		LogLevel:        strings.ToLower(envOr("PIPELINEIQ_LOG_LEVEL", "info")),
		IngestKey:       strings.TrimSpace(os.Getenv("PIPELINEIQ_INGEST_KEY")),
		Rules:           rules.Defaults(),
	}
	if err := applyRulesFile(&cfg.Rules, strings.TrimSpace(os.Getenv("PIPELINEIQ_RULES_FILE"))); err != nil {
		return Config{}, err
	}
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("PIPELINEIQ_DATABASE_URL is required")
	}
	if err := overrideDuration(&cfg.ShutdownTimeout, "PIPELINEIQ_SHUTDOWN_TIMEOUT"); err != nil {
		return Config{}, err
	}
	if err := overrideDuration(&cfg.ReadyTimeout, "PIPELINEIQ_READY_TIMEOUT"); err != nil {
		return Config{}, err
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return Config{}, fmt.Errorf("PIPELINEIQ_LOG_LEVEL must be debug, info, warn, or error")
	}
	// Last, so a bad duration or log level is still the error the caller sees.
	if cfg.IngestKey == "" {
		return Config{}, fmt.Errorf("PIPELINEIQ_INGEST_KEY is required")
	}
	return cfg, nil
}

func overrideDuration(dst *time.Duration, key string) error {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if d <= 0 {
		return fmt.Errorf("%s must be greater than 0", key)
	}
	*dst = d
	return nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
