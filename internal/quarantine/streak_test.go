package quarantine

import "testing"

func TestStreak(t *testing.T) {
	runs := []StageRun{
		{BuildID: 5, Outcome: "PASSED"},
		{BuildID: 6, Outcome: "PASSED"},
		{BuildID: 7, Outcome: "FAILED"},
		{BuildID: 8, Outcome: "PASSED"},
		{BuildID: 9, Outcome: "PASSED"},
		{BuildID: 9, Outcome: "PASSED"},
	}
	if got := Streak(runs, 4); got != 2 {
		t.Fatalf("streak from the newest = %d, want 2", got)
	}
	if got := Streak(runs, 7); got != 2 {
		t.Fatalf("streak after the failure = %d, want 2", got)
	}
	if got := Streak(runs, 9); got != 0 {
		t.Fatalf("nothing after the creating build = %d", got)
	}
	skipped := []StageRun{{BuildID: 2, Outcome: "SKIPPED"}, {BuildID: 3, Outcome: "PASSED"}}
	if got := Streak(skipped, 1); got != 1 {
		t.Fatalf("a pass after an older skip counts, got %d", got)
	}
	newestSkip := []StageRun{{BuildID: 2, Outcome: "PASSED"}, {BuildID: 3, Outcome: "SKIPPED"}}
	if got := Streak(newestSkip, 1); got != 0 {
		t.Fatalf("a skip as the newest run breaks the streak, got %d", got)
	}
}
