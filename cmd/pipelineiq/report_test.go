package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReportCases(t *testing.T) {
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "core", "target", "surefire-reports", "TEST-Core.xml")
	if err := os.MkdirAll(filepath.Dir(xmlPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xmlPath, []byte(`<testsuite><testcase name="adds" classname="c" time="0.001"/></testsuite>`), 0o644); err != nil {
		t.Fatal(err)
	}
	stages := filepath.Join(dir, "stages.json")
	if err := os.WriteFile(stages, []byte(`[{"name":"Build","duration_ms":10,"result":"SUCCESS"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	t.Run("success", func(t *testing.T) {
		var got atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got.Add(1)
			if r.Header.Get("Authorization") != "Bearer secret" {
				t.Errorf("auth = %q", r.Header.Get("Authorization"))
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			if !strings.Contains(r.FormValue("metadata"), `"build_number":7`) {
				t.Errorf("metadata = %s", r.FormValue("metadata"))
			}
			if !strings.Contains(r.FormValue("stages"), "Build") {
				t.Errorf("stages = %s", r.FormValue("stages"))
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		code := report(args(srv.URL, stages, xmlPath))
		if code != 0 || got.Load() != 1 {
			t.Fatalf("code = %d calls = %d", code, got.Load())
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		var got atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got.Add(1)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer srv.Close()
		code := report(args(srv.URL, stages, xmlPath))
		if code != 1 {
			t.Fatalf("code = %d, want 1", code)
		}
		if got.Load() != 1 {
			t.Fatalf("401 was retried %d times", got.Load())
		}
	})

	t.Run("retry then success", func(t *testing.T) {
		var got atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got.Add(1) == 1 {
				http.Error(w, "down", http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()
		cfg := mustParse(t, args(srv.URL, stages, xmlPath))
		cfg.Backoff = time.Millisecond
		cfg.Stderr = io.Discard
		if err := cfg.upload(t.Context()); err != nil {
			t.Fatal(err)
		}
		if got.Load() != 2 {
			t.Fatalf("calls = %d, want 2", got.Load())
		}
	})

	t.Run("server down", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		url := srv.URL
		srv.Close()
		code := report(append(args(url, stages, xmlPath), "--strict=false"))
		if code != 0 {
			t.Fatalf("non-strict code = %d, want 0", code)
		}
		code = report(append(args(url, stages, xmlPath), "--strict"))
		if code != 1 {
			t.Fatalf("strict code = %d, want 1", code)
		}
	})
}

func TestQuarantineRequiresArgs(t *testing.T) {
	if code := runQuarantine(nil); code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
}

func args(server, stages, report string) []string {
	return []string{
		"--server", server,
		"--key", "secret",
		"--repository", "nibinrj/pipelineiq-lab",
		"--job", "pipelineiq-lab/main",
		"--build", "7",
		"--branch", "main",
		"--commit", "abc123",
		"--result", "SUCCESS",
		"--agent", "agent",
		"--stages", stages,
		"--report", report,
	}
}

func mustParse(t *testing.T, args []string) reportConfig {
	t.Helper()
	cfg, err := parseReport(args, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	cfg.HTTP = &http.Client{Timeout: 2 * time.Second}
	return cfg
}
