package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type reportConfig struct {
	Server     string
	Key        string
	Repository string
	Job        string
	Build      int
	Branch     string
	PR         int
	Commit     string
	Result     string
	Agent      string
	Stages     string
	LogTail    string
	Reports    []string
	Strict     bool
	Attempts   int
	Backoff    time.Duration
	HTTP       *http.Client
	Stderr     io.Writer
}

func report(args []string) int {
	cfg, err := parseReport(args, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := cfg.upload(context.Background()); err != nil {
		if !cfg.Strict && isDown(err) {
			_, _ = fmt.Fprintf(cfg.Stderr, "pipelineiq: server unreachable, report not uploaded: %v\n", err)
			return 0
		}
		_, _ = fmt.Fprintf(cfg.Stderr, "pipelineiq: %v\n", err)
		return 1
	}
	return 0
}

func parseReport(args []string, stderr io.Writer) (reportConfig, error) {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfg := reportConfig{Stderr: stderr, Attempts: 3, Backoff: 200 * time.Millisecond}
	fs.StringVar(&cfg.Server, "server", "", "PipelineIQ base URL")
	fs.StringVar(&cfg.Key, "key", "", "ingest API key")
	fs.StringVar(&cfg.Repository, "repository", "", "repository name, for example nibinrj/pipelineiq-lab")
	fs.StringVar(&cfg.Job, "job", "", "Jenkins job name")
	fs.IntVar(&cfg.Build, "build", 0, "Jenkins build number")
	fs.StringVar(&cfg.Branch, "branch", "", "branch name")
	fs.IntVar(&cfg.PR, "pr", 0, "pull request number, if this build is a PR")
	fs.StringVar(&cfg.Commit, "commit", "", "commit SHA")
	fs.StringVar(&cfg.Result, "result", "", "Jenkins result: SUCCESS, UNSTABLE, FAILURE, NOT_BUILT, ABORTED")
	fs.StringVar(&cfg.Agent, "agent", "", "agent name")
	fs.StringVar(&cfg.Stages, "stages", "stages.json", "stage timing JSON file")
	fs.StringVar(&cfg.LogTail, "log-tail", "pipelineiq-log-tail.txt", "log tail file")
	fs.BoolVar(&cfg.Strict, "strict", false, "exit non-zero if the server is unreachable")
	var reports multiFlag
	fs.Var(&reports, "report", "report file or directory (repeatable; default searches Surefire and Failsafe dirs)")
	if err := fs.Parse(args); err != nil {
		return reportConfig{}, err
	}
	cfg.Reports = reports
	if cfg.Server == "" || cfg.Key == "" || cfg.Repository == "" || cfg.Job == "" || cfg.Branch == "" || cfg.Commit == "" || cfg.Result == "" || cfg.Build <= 0 {
		return reportConfig{}, fmt.Errorf("server, key, repository, job, build, branch, commit, and result are required")
	}
	return cfg, nil
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func (cfg reportConfig) upload(ctx context.Context) error {
	body, contentType, err := cfg.body()
	if err != nil {
		return err
	}
	client := cfg.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	attempts := cfg.Attempts
	if attempts <= 0 {
		attempts = 3
	}
	var last error
	for attempt := 1; attempt <= attempts; attempt++ {
		err := postOnce(ctx, client, cfg.Server, cfg.Key, contentType, body)
		if err == nil {
			return nil
		}
		if errors.Is(err, errUnauthorized) || errors.Is(err, errBadRequest) {
			return err
		}
		last = err
		if attempt == attempts {
			break
		}
		timer := time.NewTimer(cfg.Backoff << (attempt - 1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return last
}

var (
	errUnauthorized = errors.New("unauthorized")
	errBadRequest   = errors.New("rejected")
	errUnavailable  = errors.New("unavailable")
)

func postOnce(ctx context.Context, client *http.Client, server, key, contentType string, body []byte) error {
	url := strings.TrimRight(server, "/") + "/api/v1/builds"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return errUnauthorized
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return fmt.Errorf("%w: %s", errBadRequest, strings.TrimSpace(string(respBody)))
	default:
		return fmt.Errorf("%w: status %d", errUnavailable, resp.StatusCode)
	}
}

func isDown(err error) bool {
	if err == nil || errors.Is(err, errUnauthorized) || errors.Is(err, errBadRequest) {
		return false
	}
	return true
}

func (cfg reportConfig) body() ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	meta := map[string]any{
		"repository":   cfg.Repository,
		"job_name":     cfg.Job,
		"build_number": cfg.Build,
		"branch":       cfg.Branch,
		"commit_sha":   cfg.Commit,
		"result":       cfg.Result,
		"agent_name":   cfg.Agent,
	}
	if cfg.PR > 0 {
		meta["pr_number"] = cfg.PR
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return nil, "", err
	}
	if err := w.WriteField("metadata", string(raw)); err != nil {
		return nil, "", err
	}
	if err := addFileField(w, "stages", cfg.Stages); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	if err := addFilePart(w, "log_tail", cfg.LogTail); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, "", err
	}
	paths, err := collectReports(cfg.Reports)
	if err != nil {
		return nil, "", err
	}
	for _, path := range paths {
		if err := addFilePart(w, "report", path); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

func addFileField(w *multipart.Writer, field, path string) error {
	if path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return w.WriteField(field, string(b))
}

func addFilePart(w *multipart.Writer, field, path string) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	part, err := w.CreateFormFile(field, filepath.ToSlash(path))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, f)
	return err
}

func collectReports(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		patterns = defaultReportDirs()
	}
	var out []string
	seen := map[string]bool{}
	for _, pattern := range patterns {
		matches, err := expandReport(pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			if !seen[match] {
				seen[match] = true
				out = append(out, match)
			}
		}
	}
	return out, nil
}

func defaultReportDirs() []string {
	dirs := []string{
		filepath.Join("target", "surefire-reports"),
		filepath.Join("target", "failsafe-reports"),
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		return dirs
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dirs = append(dirs,
			filepath.Join(entry.Name(), "target", "surefire-reports"),
			filepath.Join(entry.Name(), "target", "failsafe-reports"),
		)
	}
	return dirs
}

func expandReport(pattern string) ([]string, error) {
	info, err := os.Stat(pattern)
	if err == nil && info.IsDir() {
		matches, err := filepath.Glob(filepath.Join(pattern, "*.xml"))
		return matches, err
	}
	if strings.ContainsAny(pattern, "*?[") {
		return filepath.Glob(pattern)
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return []string{pattern}, nil
}
