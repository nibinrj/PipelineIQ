// Package rules is the flaky detection. It is a pure function of stored runs.
// Infra attribution has already marked the builds. This package does not read logs.
package rules

import "encoding/json"

const (
	RuleR1 = "R1"
	RuleR2 = "R2"
	RuleR3 = "R3"
)

// Outcomes stored on test_run. They match the check constraint.
const (
	Passed  = "PASSED"
	Failed  = "FAILED"
	Error   = "ERROR"
	Skipped = "SKIPPED"
	Flaky   = "FLAKY"
)

// Thresholds are the plan defaults unless a rules file overrides them.
type Thresholds struct {
	R1MinFlaky    int
	R1Window      int
	R3MinFlips    int
	R3Window      int
	ReleasePasses int
	CapPercent    int
	CapMax        int
}

// Defaults are the plan's numbers. ReleasePasses and the cap are used by quarantine.
func Defaults() Thresholds {
	return Thresholds{
		R1MinFlaky:    2,
		R1Window:      30,
		R3MinFlips:    3,
		R3Window:      30,
		ReleasePasses: 10,
		CapPercent:    5,
		CapMax:        10,
	}
}

// Run is one stored result for one test. Oldest first when Match walks a sequence.
type Run struct {
	BuildID   int64
	CommitSHA string
	Branch    string
	Infra     bool
	Outcome   string
	Stage     string
}

// Hit is the first rule that fired, with the runs that explain it.
type Hit struct {
	Rule     string
	Evidence []byte
}

type evidenceRun struct {
	BuildID   int64  `json:"build_id"`
	Outcome   string `json:"outcome"`
	CommitSHA string `json:"commit_sha,omitempty"`
}

type evidenceDoc struct {
	Rule         string        `json:"rule"`
	SinceBuildID int64         `json:"since_build_id"`
	Runs         []evidenceRun `json:"runs"`
}

// Match returns the first of R1, R2, R3. windowBuildIDs is the last R1Window
// non-infra builds of the repository. defaultBranch is repository.default_branch.
func Match(runs []Run, windowBuildIDs map[int64]bool, defaultBranch string, th Thresholds) (Hit, bool) {
	if th.R1MinFlaky <= 0 {
		th.R1MinFlaky = Defaults().R1MinFlaky
	}
	if hit, ok := matchR1(runs, windowBuildIDs, th.R1MinFlaky); ok {
		return hit, true
	}
	if hit, ok := matchR2(runs); ok {
		return hit, true
	}
	if th.R3MinFlips <= 0 {
		th.R3MinFlips = Defaults().R3MinFlips
	}
	if th.R3Window <= 0 {
		th.R3Window = Defaults().R3Window
	}
	if hit, ok := matchR3(runs, defaultBranch, th.R3MinFlips, th.R3Window); ok {
		return hit, true
	}
	return Hit{}, false
}

// WindowIDs returns the newest non-infra build ids, up to n. newest is the
// highest id. The map is what Match expects.
func WindowIDs(buildIDs []int64, infra map[int64]bool, n int) map[int64]bool {
	out := map[int64]bool{}
	if n <= 0 {
		return out
	}
	kept := 0
	for i := len(buildIDs) - 1; i >= 0 && kept < n; i-- {
		id := buildIDs[i]
		if infra[id] {
			continue
		}
		out[id] = true
		kept++
	}
	return out
}

func matchR1(runs []Run, window map[int64]bool, minFlaky int) (Hit, bool) {
	seen := map[int64]bool{}
	var evidence []evidenceRun
	for _, run := range runs {
		if run.Infra || !window[run.BuildID] || run.Outcome != Flaky || seen[run.BuildID] {
			continue
		}
		seen[run.BuildID] = true
		evidence = append(evidence, evidenceRun{BuildID: run.BuildID, Outcome: run.Outcome, CommitSHA: run.CommitSHA})
	}
	if len(evidence) < minFlaky {
		return Hit{}, false
	}
	return hit(RuleR1, evidence), true
}

func matchR2(runs []Run) (Hit, bool) {
	type sides struct {
		pass bool
		fail bool
		rows []evidenceRun
	}
	byCommit := map[string]*sides{}
	for _, run := range runs {
		if run.Infra || run.CommitSHA == "" {
			continue
		}
		side := byCommit[run.CommitSHA]
		if side == nil {
			side = &sides{}
			byCommit[run.CommitSHA] = side
		}
		switch run.Outcome {
		case Passed:
			side.pass = true
			side.rows = append(side.rows, evidenceRun{BuildID: run.BuildID, Outcome: run.Outcome, CommitSHA: run.CommitSHA})
		case Failed, Error:
			side.fail = true
			side.rows = append(side.rows, evidenceRun{BuildID: run.BuildID, Outcome: run.Outcome, CommitSHA: run.CommitSHA})
		}
	}
	for _, side := range byCommit {
		if side.pass && side.fail {
			return hit(RuleR2, side.rows), true
		}
	}
	return Hit{}, false
}

func matchR3(runs []Run, defaultBranch string, minFlips, window int) (Hit, bool) {
	var kept []Run
	for _, run := range runs {
		if run.Infra || run.Branch != defaultBranch || run.Outcome == Skipped {
			continue
		}
		kept = append(kept, run)
	}
	if len(kept) < window {
		return Hit{}, false
	}
	kept = kept[len(kept)-window:]
	flips := 0
	for i := 1; i < len(kept); i++ {
		if class(kept[i-1].Outcome) != class(kept[i].Outcome) {
			flips++
		}
	}
	if flips < minFlips {
		return Hit{}, false
	}
	rows := make([]evidenceRun, 0, len(kept))
	for _, run := range kept {
		rows = append(rows, evidenceRun{BuildID: run.BuildID, Outcome: run.Outcome, CommitSHA: run.CommitSHA})
	}
	return hit(RuleR3, rows), true
}

func class(outcome string) string {
	switch outcome {
	case Failed, Error:
		return "fail"
	case Flaky:
		return "flaky"
	default:
		return outcome
	}
}

func hit(rule string, runs []evidenceRun) Hit {
	since := int64(0)
	for _, run := range runs {
		if run.BuildID > since {
			since = run.BuildID
		}
	}
	raw, err := json.Marshal(evidenceDoc{Rule: rule, SinceBuildID: since, Runs: runs})
	if err != nil {
		raw = []byte(`{"rule":"` + rule + `"}`)
	}
	return Hit{Rule: rule, Evidence: raw}
}
