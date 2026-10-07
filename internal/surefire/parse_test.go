package surefire

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixtures(t *testing.T) {
	tests := []struct {
		file       string
		wantN      int
		wantOut    string
		wantReruns int32
		wantType   string
	}{
		{file: "passed.xml", wantN: 1, wantOut: Passed},
		{file: "failed.xml", wantN: 1, wantOut: Failed, wantType: "java.lang.AssertionError"},
		{file: "error.xml", wantN: 1, wantOut: Error, wantType: "java.lang.IllegalStateException"},
		{file: "skipped.xml", wantN: 1, wantOut: Skipped},
		{file: "flaky-failure.xml", wantN: 1, wantOut: Flaky, wantReruns: 2, wantType: "java.lang.AssertionError"},
		{file: "flaky-error.xml", wantN: 1, wantOut: Flaky, wantReruns: 1, wantType: "java.lang.IllegalStateException"},
		{file: "rerun-failure.xml", wantN: 1, wantOut: Failed, wantReruns: 2, wantType: "java.lang.AssertionError"},
		{file: "wrong-totals.xml", wantN: 1, wantOut: Failed, wantReruns: 5, wantType: "java.lang.AssertionError"},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "surefire", tt.file))
			if err != nil {
				t.Fatal(err)
			}
			got, err := ParseBytes(body)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.wantN {
				t.Fatalf("len = %d, want %d", len(got), tt.wantN)
			}
			one := got[0]
			if one.Outcome != tt.wantOut {
				t.Errorf("outcome = %s, want %s", one.Outcome, tt.wantOut)
			}
			if one.RerunFailures != tt.wantReruns {
				t.Errorf("reruns = %d, want %d", one.RerunFailures, tt.wantReruns)
			}
			if tt.wantType != "" && one.FailureType != tt.wantType {
				t.Errorf("type = %s, want %s", one.FailureType, tt.wantType)
			}
			if tt.wantOut == Failed || tt.wantOut == Error || tt.wantOut == Flaky {
				if one.FailureHash == "" {
					t.Error("missing failure hash")
				}
			}
		})
	}
}

func TestFailureHashIgnoresFrameworkFrames(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<testsuite tests="99" failures="99">
  <testcase name="adds" classname="io.pipelineiq.lab.core.CoreTest" time="0.004">
    <failure message="expected 3" type="java.lang.AssertionError">java.lang.AssertionError: expected 3
	at org.junit.jupiter.api.AssertionUtils.fail(AssertionUtils.java:38)
	at io.pipelineiq.lab.core.CoreTest.adds(CoreTest.java:12)
	at java.base/java.lang.reflect.Method.invoke(Method.java:580)
</failure>
  </testcase>
</testsuite>`)
	got, err := ParseBytes(body)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].DurationMs != 4 {
		t.Errorf("duration = %d, want 4", got[0].DurationMs)
	}
	if got[0].FailureHash == "" {
		t.Fatal("hash empty")
	}
	again, err := ParseBytes(body)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].FailureHash != again[0].FailureHash {
		t.Fatalf("hash changed between parses")
	}
}

func TestModuleFromFilename(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "core/target/surefire-reports/TEST-Foo.xml", want: "core"},
		{in: "target/surefire-reports/TEST-Foo.xml", want: ""},
		{in: `api\target\failsafe-reports\TEST-Bar.xml`, want: "api"},
	}
	for _, tt := range tests {
		if got := ModuleFromFilename(tt.in); got != tt.want {
			t.Errorf("ModuleFromFilename(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
