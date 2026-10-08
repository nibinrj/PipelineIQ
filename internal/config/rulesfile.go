package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/nibinrj/PipelineIQ/internal/rules"
)

// applyRulesFile overlays a flat key: value file onto the plan defaults.
// The file is optional. A missing path is not an error. Nested YAML is rejected
// so this parser cannot silently ignore a structure it does not understand.
func applyRulesFile(th *rules.Thresholds, path string) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("PIPELINEIQ_RULES_FILE: %w", err)
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok || strings.Contains(value, ":") {
			return fmt.Errorf("PIPELINEIQ_RULES_FILE:%d: expected key: value", lineNo)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("PIPELINEIQ_RULES_FILE:%d: %s must be a positive integer", lineNo, key)
		}
		switch key {
		case "r1_flaky_builds":
			th.R1MinFlaky = n
		case "r1_window":
			th.R1Window = n
		case "r3_flips":
			th.R3MinFlips = n
		case "r3_window":
			th.R3Window = n
		case "release_passes":
			th.ReleasePasses = n
		case "cap_percent":
			th.CapPercent = n
		case "cap_max":
			th.CapMax = n
		default:
			return fmt.Errorf("PIPELINEIQ_RULES_FILE:%d: unknown key %s", lineNo, key)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("PIPELINEIQ_RULES_FILE: %w", err)
	}
	return nil
}
