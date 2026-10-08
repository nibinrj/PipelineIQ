package quarantine

import (
	"strings"
	"testing"
)

func TestFormats(t *testing.T) {
	tests := []Ref{
		{ClassName: "com.example.lab.FlakyTest", MethodName: "testTimeBound"},
		{ClassName: "com.example.lab.FlakyTest", MethodName: "testFlips"},
		{ClassName: "com.example.Other", MethodName: "m"},
	}
	excludes := ExcludesFile(tests)
	if !strings.HasPrefix(excludes, "# pipelineiq quarantine") {
		t.Fatalf("header missing: %q", excludes)
	}
	if !strings.Contains(excludes, "com.example.Other#m\n") {
		t.Fatalf("excludes = %q", excludes)
	}
	if !strings.Contains(excludes, "com.example.lab.FlakyTest#testFlips\n") {
		t.Fatalf("method order = %q", excludes)
	}
	only := OnlyArg(tests)
	want := "com.example.Other#m,com.example.lab.FlakyTest#testFlips+testTimeBound"
	if only != want {
		t.Fatalf("only = %q, want %q", only, want)
	}
	if OnlyArg(nil) != "" {
		t.Fatal("empty only-list must be empty")
	}
	if strings.Contains(ExcludesFile(nil), "#") && strings.Count(ExcludesFile(nil), "\n") != 1 {
		t.Fatalf("empty excludes = %q", ExcludesFile(nil))
	}
}
