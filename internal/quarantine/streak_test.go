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
	if got := Streak(skipped, 1); got != 0 {
		t.Fatalf("skip breaks the streak, got %d", got)
	}
}
