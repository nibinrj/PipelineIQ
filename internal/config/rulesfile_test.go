package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nibinrj/PipelineIQ/internal/rules"
)

func TestRulesFileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.txt")
	body := "# comment\nr1_flaky_builds: 4\nrelease_passes: 3\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	th := rules.Defaults()
	if err := applyRulesFile(&th, path); err != nil {
		t.Fatal(err)
	}
	if th.R1MinFlaky != 4 || th.ReleasePasses != 3 || th.R1Window != 30 {
		t.Fatalf("thresholds = %+v", th)
	}
}

func TestRulesFileRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rules.txt")
	if err := os.WriteFile(path, []byte("floor: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	th := rules.Defaults()
	if err := applyRulesFile(&th, path); err == nil {
		t.Fatal("unknown key was accepted")
	}
}
