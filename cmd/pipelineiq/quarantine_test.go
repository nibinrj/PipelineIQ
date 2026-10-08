package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuarantineFormats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		if r.URL.Query().Get("repo") != "nibinrj/pipelineiq-lab" {
			http.Error(w, "repo", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"repository":"nibinrj/pipelineiq-lab","active":[{"class_name":"com.example.lab.FlakyTest","method_name":"testFlips"},{"class_name":"com.example.lab.FlakyTest","method_name":"testTimeBound"}]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	excludes := filepath.Join(dir, "excludes.txt")
	if code := quarantine([]string{"--server", srv.URL, "--key", "key", "--repo", "nibinrj/pipelineiq-lab", "--format", "surefire-excludes", "--out", excludes}); code != 0 {
		t.Fatalf("excludes exit %d", code)
	}
	got, err := os.ReadFile(excludes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "com.example.lab.FlakyTest#testFlips\n") || !strings.Contains(string(got), "# pipelineiq quarantine") {
		t.Fatalf("excludes = %s", got)
	}

	only := filepath.Join(dir, "only.txt")
	if code := quarantine([]string{"--server", srv.URL, "--key", "key", "--repo", "nibinrj/pipelineiq-lab", "--format", "surefire-only", "--out", only}); code != 0 {
		t.Fatalf("only exit %d", code)
	}
	got, err = os.ReadFile(only)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "com.example.lab.FlakyTest#testFlips+testTimeBound" {
		t.Fatalf("only = %q", got)
	}
}

func TestQuarantineUnreachableWritesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "excludes.txt")
	code := quarantine([]string{
		"--server", "http://127.0.0.1:1",
		"--key", "key",
		"--repo", "nibinrj/pipelineiq-lab",
		"--format", "surefire-excludes",
		"--out", out,
	})
	if code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("unreachable file = %q, want empty", got)
	}
}

func TestQuarantineUnauthorizedDoesNotWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusUnauthorized)
	}))
	defer srv.Close()
	dir := t.TempDir()
	out := filepath.Join(dir, "excludes.txt")
	code := quarantine([]string{"--server", srv.URL, "--key", "bad", "--repo", "nibinrj/pipelineiq-lab", "--format", "surefire-only", "--out", out})
	if code == 0 {
		t.Fatal("401 exited 0")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("401 wrote a file: %v", err)
	}
}
