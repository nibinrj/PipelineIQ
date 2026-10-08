package rules

import (
	"strconv"
	"testing"
)

func run(id int64, commit, branch, outcome string, infra bool) Run {
	return Run{BuildID: id, CommitSHA: commit, Branch: branch, Outcome: outcome, Infra: infra, Stage: "BLOCKING"}
}

func TestR1NeedsTwoFlakyBuilds(t *testing.T) {
	th := Defaults()
	window := map[int64]bool{1: true, 2: true, 3: true}
	one := []Run{run(1, "a", "main", Flaky, false), run(2, "b", "main", Passed, false)}
	if _, ok := Match(one, window, "main", th); ok {
		t.Fatal("one FLAKY build must not fire")
	}
	two := []Run{
		run(1, "a", "main", Flaky, false),
		run(2, "b", "main", Flaky, false),
	}
	hit, ok := Match(two, window, "main", th)
	if !ok || hit.Rule != RuleR1 {
		t.Fatalf("two FLAKY builds = %+v %v", hit, ok)
	}
}

func TestR1IgnoresInfraAndBuildsOutsideTheWindow(t *testing.T) {
	th := Defaults()
	window := map[int64]bool{2: true, 3: true}
	runs := []Run{
		run(1, "old", "main", Flaky, false),
		run(2, "infra", "main", Flaky, true),
		run(3, "ok", "main", Passed, false),
	}
	if _, ok := Match(runs, window, "main", th); ok {
		t.Fatal("infra and out-of-window FLAKY runs must not fire R1")
	}
}

func TestR2SameCommitAndTheEdges(t *testing.T) {
	th := Defaults()
	window := map[int64]bool{}
	tests := []struct {
		name string
		runs []Run
		want bool
	}{
		{
			name: "pass and fail",
			runs: []Run{run(1, "abc", "main", Passed, false), run(2, "abc", "feature", Failed, false)},
			want: true,
		},
		{
			name: "pass and error",
			runs: []Run{run(1, "abc", "main", Passed, false), run(2, "abc", "main", Error, false)},
			want: true,
		},
		{
			name: "two failures are a break",
			runs: []Run{run(1, "abc", "main", Failed, false), run(2, "abc", "main", Failed, false)},
			want: false,
		},
		{
			name: "pass and skip",
			runs: []Run{run(1, "abc", "main", Passed, false), run(2, "abc", "main", Skipped, false)},
			want: false,
		},
		{
			name: "different commits",
			runs: []Run{run(1, "abc", "main", Passed, false), run(2, "def", "main", Failed, false)},
			want: false,
		},
		{
			name: "fail only on infra",
			runs: []Run{run(1, "abc", "main", Passed, false), run(2, "abc", "main", Failed, true)},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, ok := Match(tt.runs, window, "main", th)
			if ok != tt.want {
				t.Fatalf("Match() ok = %v, want %v, hit %+v", ok, tt.want, hit)
			}
			if ok && hit.Rule != RuleR2 {
				t.Fatalf("rule = %s, want R2", hit.Rule)
			}
		})
	}
}

func TestR3BreakThenFixIsNotFlaky(t *testing.T) {
	th := Defaults()
	var runs []Run
	for i := int64(1); i <= 27; i++ {
		runs = append(runs, run(i, "c", "main", Passed, false))
	}
	runs = append(runs,
		run(28, "broken", "main", Failed, false),
		run(29, "broken", "main", Failed, false),
		run(30, "fixed", "main", Passed, false),
	)
	window := map[int64]bool{}
	if _, ok := Match(runs, window, "main", th); ok {
		t.Fatal("a break then a fix is two changes, not three")
	}
}

func TestR3FiresOnThreeFlips(t *testing.T) {
	th := Defaults()
	var runs []Run
	outcomes := []string{Passed, Failed, Passed, Failed, Passed}
	for i := 0; i < 25; i++ {
		runs = append(runs, run(int64(i+1), "c"+strconv.Itoa(i+1), "main", Passed, false))
	}
	for i, outcome := range outcomes {
		runs = append(runs, run(int64(26+i), "flip"+strconv.Itoa(i), "main", outcome, false))
	}
	hit, ok := Match(runs, map[int64]bool{}, "main", th)
	if !ok || hit.Rule != RuleR3 {
		t.Fatalf("Match() = %+v %v", hit, ok)
	}
}

func TestR3IgnoresOtherBranchesSkipsAndInfra(t *testing.T) {
	th := Defaults()
	var runs []Run
	for i := int64(1); i <= 30; i++ {
		outcome := Passed
		if i%2 == 0 {
			outcome = Failed
		}
		runs = append(runs, run(i, "feature-"+strconv.FormatInt(i, 10), "feature", outcome, false))
	}
	for i := int64(31); i <= 40; i++ {
		runs = append(runs, run(i, "infra-"+strconv.FormatInt(i, 10), "main", Failed, true))
	}
	for i := int64(41); i <= 50; i++ {
		runs = append(runs, run(i, "skip-"+strconv.FormatInt(i, 10), "main", Skipped, false))
	}
	for i := int64(51); i <= 55; i++ {
		runs = append(runs, run(i, "main-"+strconv.FormatInt(i, 10), "main", Passed, false))
	}
	if _, ok := Match(runs, map[int64]bool{}, "main", th); ok {
		t.Fatal("PR flips, infra failures, and skips must not satisfy R3")
	}
}

func TestR3DoesNotFireWithFewerThanWindowRuns(t *testing.T) {
	th := Defaults()
	var runs []Run
	for i := int64(1); i <= 10; i++ {
		outcome := Passed
		if i%2 == 0 {
			outcome = Failed
		}
		runs = append(runs, run(i, "short-"+strconv.FormatInt(i, 10), "main", outcome, false))
	}
	if _, ok := Match(runs, map[int64]bool{}, "main", th); ok {
		t.Fatal("fewer than 30 kept runs must not fire")
	}
}

func TestR1WinsOverR2(t *testing.T) {
	th := Defaults()
	runs := []Run{
		run(1, "abc", "main", Flaky, false),
		run(2, "abc", "main", Passed, false),
		run(3, "abc", "main", Failed, false),
		run(4, "def", "main", Flaky, false),
	}
	window := map[int64]bool{1: true, 2: true, 3: true, 4: true}
	hit, ok := Match(runs, window, "main", th)
	if !ok || hit.Rule != RuleR1 {
		t.Fatalf("Match() = %+v %v, want R1", hit, ok)
	}
}

func TestWindowSkipsInfra(t *testing.T) {
	ids := []int64{1, 2, 3, 4}
	infra := map[int64]bool{3: true}
	got := WindowIDs(ids, infra, 2)
	if len(got) != 2 || !got[4] || !got[2] || got[3] {
		t.Fatalf("window = %v", got)
	}
}
