package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/nibinrj/PipelineIQ/internal/quarantine"
)

type quarantineConfig struct {
	Server  string
	Key     string
	Repo    string
	Format  string
	Out     string
	Stdout  io.Writer
	Stderr  io.Writer
	HTTP    *http.Client
	Timeout time.Duration
}

func runQuarantine(args []string) int {
	cfg, err := parseQuarantine(args, os.Stderr)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "pipelineiq: %v\n", err)
		return 2
	}
	if err := cfg.run(context.Background()); err != nil {
		_, _ = fmt.Fprintf(cfg.Stderr, "pipelineiq: %v\n", err)
		if cfg.unreachable(err) {
			if writeErr := cfg.write(""); writeErr != nil {
				_, _ = fmt.Fprintf(cfg.Stderr, "pipelineiq: %v\n", writeErr)
				return 1
			}
			return 0
		}
		return 2
	}
	return 0
}

func parseQuarantine(args []string, stderr io.Writer) (quarantineConfig, error) {
	fs := flag.NewFlagSet("quarantine", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := quarantineConfig{Stderr: stderr, Stdout: os.Stdout, Timeout: 10 * time.Second}
	fs.StringVar(&cfg.Server, "server", "", "PipelineIQ base URL")
	fs.StringVar(&cfg.Key, "key", "", "ingest API key")
	fs.StringVar(&cfg.Repo, "repo", "", "repository name, for example nibinrj/pipelineiq-lab")
	fs.StringVar(&cfg.Format, "format", "", "surefire-excludes or surefire-only")
	fs.StringVar(&cfg.Out, "out", "", "output file; stdout if omitted")
	if err := fs.Parse(args); err != nil {
		return quarantineConfig{}, err
	}
	if cfg.Server == "" || cfg.Key == "" || cfg.Repo == "" {
		return quarantineConfig{}, fmt.Errorf("server, key, and repo are required")
	}
	switch cfg.Format {
	case "surefire-excludes", "surefire-only":
	default:
		return quarantineConfig{}, fmt.Errorf("format must be surefire-excludes or surefire-only")
	}
	return cfg, nil
}

func (cfg quarantineConfig) run(ctx context.Context) error {
	list, err := cfg.fetch(ctx)
	if err != nil {
		return err
	}
	refs := make([]quarantine.Ref, 0, len(list.Active))
	for _, item := range list.Active {
		refs = append(refs, quarantine.Ref{ClassName: item.ClassName, MethodName: item.MethodName})
	}
	var text string
	if cfg.Format == "surefire-excludes" {
		text = quarantine.ExcludesFile(refs)
	} else {
		text = quarantine.OnlyArg(refs)
		if text != "" {
			text += "\n"
		}
	}
	return cfg.write(text)
}

func (cfg quarantineConfig) fetch(ctx context.Context) (quarantine.List, error) {
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: cfg.Timeout}
	}
	endpoint, err := url.Parse(strings.TrimRight(cfg.Server, "/") + "/api/v1/quarantine")
	if err != nil {
		return quarantine.List{}, err
	}
	query := endpoint.Query()
	query.Set("repo", cfg.Repo)
	endpoint.RawQuery = query.Encode()
	reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return quarantine.List{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Key)
	resp, err := client.Do(req)
	if err != nil {
		return quarantine.List{}, fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return quarantine.List{}, err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return quarantine.List{}, fmt.Errorf("unauthorized")
	}
	if resp.StatusCode >= 500 {
		return quarantine.List{}, fmt.Errorf("connect: status %d", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return quarantine.List{}, fmt.Errorf("rejected: %s", strings.TrimSpace(string(body)))
	}
	var list quarantine.List
	if err := json.Unmarshal(body, &list); err != nil {
		return quarantine.List{}, fmt.Errorf("response: %w", err)
	}
	return list, nil
}

func (cfg quarantineConfig) unreachable(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "connect:") || strings.Contains(text, "connection refused") || strings.Contains(text, "timeout") || strings.Contains(text, "no such host")
}

func (cfg quarantineConfig) write(text string) error {
	if cfg.Out == "" {
		_, err := io.WriteString(cfg.Stdout, text)
		return err
	}
	return os.WriteFile(cfg.Out, []byte(text), 0o644)
}
