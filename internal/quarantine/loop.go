package quarantine

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nibinrj/PipelineIQ/internal/infra"
	"github.com/nibinrj/PipelineIQ/internal/rules"
	"github.com/nibinrj/PipelineIQ/internal/store"
)

// AfterIngest attributes the build, updates release streaks, then applies R1-R3.
// It uses the caller's transaction. A failure rolls the ingest back.
func AfterIngest(ctx context.Context, q *store.Queries, repoID, buildID int64, logTail string, th rules.Thresholds) error {
	if th.ReleasePasses <= 0 {
		th = rules.Defaults()
	}
	reason, infraHit := infra.Attribute(logTail)
	var reasonPtr *string
	if infraHit {
		text := string(reason)
		reasonPtr = &text
	}
	if err := q.SetBuildInfra(ctx, buildID, infraHit, reasonPtr); err != nil {
		return fmt.Errorf("infra: %w", err)
	}
	if err := releaseReady(ctx, q, repoID, th.ReleasePasses); err != nil {
		return err
	}
	return detect(ctx, q, repoID, buildID, th)
}

func releaseReady(ctx context.Context, q *store.Queries, repoID int64, need int) error {
	active, err := q.ListActiveQuarantineByRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("active quarantine: %w", err)
	}
	if len(active) == 0 {
		return nil
	}
	runs, err := q.ListTestRunsForRules(ctx, repoID)
	if err != nil {
		return fmt.Errorf("runs: %w", err)
	}
	byCase := map[int64][]StageRun{}
	for _, run := range runs {
		if run.Stage != "QUARANTINE" {
			continue
		}
		byCase[run.TestCaseID] = append(byCase[run.TestCaseID], StageRun{BuildID: run.BuildID, Outcome: run.Outcome})
	}
	for _, row := range active {
		streak := Streak(byCase[row.TestCaseID], sinceBuildID(row.Evidence))
		if streak >= need {
			if err := q.ReleaseQuarantine(ctx, row.ID, int32(streak)); err != nil {
				return fmt.Errorf("release %d: %w", row.ID, err)
			}
			continue
		}
		if int(row.ConsecutivePasses) == streak {
			continue
		}
		if err := q.UpdateQuarantinePasses(ctx, row.ID, int32(streak)); err != nil {
			return fmt.Errorf("passes %d: %w", row.ID, err)
		}
	}
	return nil
}

func detect(ctx context.Context, q *store.Queries, repoID, buildID int64, th rules.Thresholds) error {
	defaultBranch, err := q.GetRepositoryDefaultBranch(ctx, repoID)
	if err != nil {
		return fmt.Errorf("default branch: %w", err)
	}
	builds, err := q.ListBuildIDsForRules(ctx, repoID)
	if err != nil {
		return fmt.Errorf("builds: %w", err)
	}
	ids := make([]int64, 0, len(builds))
	infraByID := map[int64]bool{}
	for _, build := range builds {
		ids = append(ids, build.ID)
		infraByID[build.ID] = build.InfraFailure
	}
	window := rules.WindowIDs(ids, infraByID, th.R1Window)
	runs, err := q.ListTestRunsForRules(ctx, repoID)
	if err != nil {
		return fmt.Errorf("runs: %w", err)
	}
	byCase := map[int64][]rules.Run{}
	for _, run := range runs {
		byCase[run.TestCaseID] = append(byCase[run.TestCaseID], rules.Run{
			BuildID:   run.BuildID,
			CommitSHA: run.CommitSha,
			Branch:    run.Branch,
			Infra:     run.InfraFailure,
			Outcome:   run.Outcome,
			Stage:     run.Stage,
		})
	}
	active, err := q.ListActiveQuarantineByRepo(ctx, repoID)
	if err != nil {
		return fmt.Errorf("active quarantine: %w", err)
	}
	activeCase := map[int64]bool{}
	for _, row := range active {
		activeCase[row.TestCaseID] = true
	}
	testCount, err := q.CountTestCases(ctx, repoID)
	if err != nil {
		return fmt.Errorf("test count: %w", err)
	}
	capN := Cap(int(testCount), th.CapPercent, th.CapMax)
	activeN := len(active)
	for caseID, history := range byCase {
		if activeCase[caseID] {
			continue
		}
		hit, ok := rules.Match(history, window, defaultBranch, th)
		if !ok {
			continue
		}
		if activeN >= capN {
			payload, err := json.Marshal(map[string]any{
				"rule":         hit.Rule,
				"test_case_id": caseID,
				"reason":       "safety cap",
				"active":       activeN,
				"cap":          capN,
			})
			if err != nil {
				return err
			}
			if err := q.InsertFlakyAlert(ctx, buildID, repoID, string(payload)); err != nil {
				return fmt.Errorf("cap alert: %w", err)
			}
			continue
		}
		inserted, err := q.InsertQuarantine(ctx, caseID, hit.Rule, string(hit.Evidence), false)
		if err != nil {
			if store.IsNoRows(err) {
				continue
			}
			return fmt.Errorf("quarantine case %d: %w", caseID, err)
		}
		if inserted.ID == 0 {
			continue
		}
		activeCase[caseID] = true
		activeN++
	}
	return nil
}

func sinceBuildID(evidence string) int64 {
	var doc struct {
		SinceBuildID int64 `json:"since_build_id"`
	}
	if err := json.Unmarshal([]byte(evidence), &doc); err != nil {
		return 0
	}
	return doc.SinceBuildID
}
