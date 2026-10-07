// Package ingest writes one build report. The same job and build number updates
// the existing row. It does not insert a second build.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nibinrj/PipelineIQ/internal/store"
	"github.com/nibinrj/PipelineIQ/internal/surefire"
)

// MaxLogTail is the most of the log that is stored. The library sends 500 lines.
// A larger upload is truncated so one report cannot fill the 64 MB container.
const MaxLogTail = 256 * 1024

// Known build results. These match the check constraint on build.result.
var knownResults = map[string]bool{
	"SUCCESS": true, "UNSTABLE": true, "FAILURE": true, "NOT_BUILT": true, "ABORTED": true,
}

// Report is one CLI upload after the XML has been parsed.
type Report struct {
	Repository  string
	JobName     string
	BuildNumber int32
	Branch      string
	PRNumber    *int32
	CommitSHA   string
	Result      string
	AgentName   string
	StartedAt   *time.Time
	FinishedAt  *time.Time
	DurationMs  *int64
	LogTail     string
	Stages      []Stage
	Tests       []Test
}

// Stage is one timedStage record.
type Stage struct {
	Name       string
	StartedAt  *time.Time
	DurationMs int64
	Result     string
}

// Test is one parsed testcase plus the module from the uploaded filename.
type Test struct {
	Module string
	surefire.Case
}

// Build is the debug view returned by GET /api/v1/builds/{id}.
type Build struct {
	ID          int64       `json:"id"`
	Repository  string      `json:"repository"`
	JobName     string      `json:"job_name"`
	BuildNumber int32       `json:"build_number"`
	Branch      string      `json:"branch"`
	PRNumber    *int32      `json:"pr_number,omitempty"`
	CommitSHA   string      `json:"commit_sha"`
	Result      string      `json:"result"`
	AgentName   string      `json:"agent_name,omitempty"`
	LogTail     string      `json:"log_tail,omitempty"`
	Stages      []StageView `json:"stages"`
	Tests       []TestView  `json:"tests"`
}

// StageView is one stored stage.
type StageView struct {
	Name       string     `json:"name"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	DurationMs int64      `json:"duration_ms"`
	Result     string     `json:"result"`
}

// TestView is one stored test run.
type TestView struct {
	Module        string `json:"module,omitempty"`
	ClassName     string `json:"class_name"`
	MethodName    string `json:"method_name"`
	Outcome       string `json:"outcome"`
	DurationMs    int64  `json:"duration_ms"`
	RerunFailures int32  `json:"rerun_failures"`
	FailureType   string `json:"failure_type,omitempty"`
	FailureHash   string `json:"failure_hash,omitempty"`
	Stage         string `json:"stage"`
}

// Service writes reports through the pool. One Apply call is one transaction.
type Service struct {
	pool *pgxpool.Pool
}

// New binds the service to the process pool.
func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Apply inserts or updates the build and replaces its stages and test runs.
func (s *Service) Apply(ctx context.Context, report Report) (Build, error) {
	if err := validate(report); err != nil {
		return Build{}, err
	}
	report.LogTail = truncate(report.LogTail, MaxLogTail)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Build{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := store.New(tx)
	repoID, err := q.GetOrCreateRepository(ctx, report.Repository)
	if err != nil {
		return Build{}, fmt.Errorf("repository: %w", err)
	}
	buildID, err := q.UpsertBuild(ctx, store.UpsertBuildParams{
		RepositoryID: repoID,
		JobName:      report.JobName,
		BuildNumber:  report.BuildNumber,
		Branch:       report.Branch,
		PrNumber:     report.PRNumber,
		CommitSha:    report.CommitSHA,
		Result:       report.Result,
		StartedAt:    report.StartedAt,
		FinishedAt:   report.FinishedAt,
		DurationMs:   report.DurationMs,
		AgentName:    optional(report.AgentName),
		LogTail:      optional(report.LogTail),
	})
	if err != nil {
		return Build{}, fmt.Errorf("build: %w", err)
	}
	if err := q.DeleteStageRunsByBuild(ctx, buildID); err != nil {
		return Build{}, fmt.Errorf("delete stages: %w", err)
	}
	if err := q.DeleteTestRunsByBuild(ctx, buildID); err != nil {
		return Build{}, fmt.Errorf("delete test runs: %w", err)
	}
	for _, stage := range report.Stages {
		if err := q.InsertStageRun(ctx, store.InsertStageRunParams{
			BuildID:    buildID,
			Name:       stage.Name,
			StartedAt:  stage.StartedAt,
			DurationMs: stage.DurationMs,
			Result:     stage.Result,
		}); err != nil {
			return Build{}, fmt.Errorf("stage %s: %w", stage.Name, err)
		}
	}
	for _, test := range report.Tests {
		caseID, err := q.UpsertTestCase(ctx, store.UpsertTestCaseParams{
			RepositoryID: repoID,
			Module:       test.Module,
			ClassName:    test.ClassName,
			MethodName:   test.MethodName,
		})
		if err != nil {
			return Build{}, fmt.Errorf("test case %s#%s: %w", test.ClassName, test.MethodName, err)
		}
		if err := q.InsertTestRun(ctx, store.InsertTestRunParams{
			BuildID:       buildID,
			TestCaseID:    caseID,
			Outcome:       test.Outcome,
			DurationMs:    test.DurationMs,
			RerunFailures: test.RerunFailures,
			FailureType:   optional(test.FailureType),
			FailureHash:   optional(test.FailureHash),
		}); err != nil {
			return Build{}, fmt.Errorf("test run %s#%s: %w", test.ClassName, test.MethodName, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Build{}, fmt.Errorf("commit: %w", err)
	}
	return s.Get(ctx, buildID)
}

// Get loads a build for the debug endpoint.
func (s *Service) Get(ctx context.Context, id int64) (Build, error) {
	q := store.New(s.pool)
	row, err := q.GetBuildByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Build{}, ErrNotFound
		}
		return Build{}, fmt.Errorf("get build: %w", err)
	}
	stages, err := q.ListStageRunsByBuild(ctx, id)
	if err != nil {
		return Build{}, fmt.Errorf("stages: %w", err)
	}
	tests, err := q.ListTestRunsByBuild(ctx, id)
	if err != nil {
		return Build{}, fmt.Errorf("tests: %w", err)
	}
	view := Build{
		ID:          row.ID,
		Repository:  row.RepositoryName,
		JobName:     row.JobName,
		BuildNumber: row.BuildNumber,
		Branch:      row.Branch,
		PRNumber:    row.PrNumber,
		CommitSHA:   row.CommitSha,
		Result:      row.Result,
		Stages:      make([]StageView, 0, len(stages)),
		Tests:       make([]TestView, 0, len(tests)),
	}
	if row.AgentName != nil {
		view.AgentName = *row.AgentName
	}
	if row.LogTail != nil {
		view.LogTail = *row.LogTail
	}
	for _, stage := range stages {
		view.Stages = append(view.Stages, StageView{
			Name:       stage.Name,
			StartedAt:  stage.StartedAt,
			DurationMs: stage.DurationMs,
			Result:     stage.Result,
		})
	}
	for _, test := range tests {
		item := TestView{
			Module:        test.Module,
			ClassName:     test.ClassName,
			MethodName:    test.MethodName,
			Outcome:       test.Outcome,
			DurationMs:    test.DurationMs,
			RerunFailures: test.RerunFailures,
			Stage:         test.Stage,
		}
		if test.FailureType != nil {
			item.FailureType = *test.FailureType
		}
		if test.FailureHash != nil {
			item.FailureHash = *test.FailureHash
		}
		view.Tests = append(view.Tests, item)
	}
	return view, nil
}

// ErrNotFound is a missing build id. The handler turns it into 404.
var ErrNotFound = errors.New("build not found")

// IsNotFound reports a missing build.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func validate(report Report) error {
	if report.Repository == "" || report.JobName == "" || report.Branch == "" || report.CommitSHA == "" {
		return fmt.Errorf("repository, job, branch, and commit are required")
	}
	if report.BuildNumber <= 0 {
		return fmt.Errorf("build number must be greater than 0")
	}
	if !knownResults[report.Result] {
		return fmt.Errorf("result %q is not a Jenkins result", report.Result)
	}
	if report.PRNumber != nil && *report.PRNumber <= 0 {
		return fmt.Errorf("pr number must be greater than 0")
	}
	if report.DurationMs != nil && *report.DurationMs < 0 {
		return fmt.Errorf("duration must be >= 0")
	}
	for _, stage := range report.Stages {
		if stage.Name == "" {
			return fmt.Errorf("stage name is required")
		}
		if stage.DurationMs < 0 {
			return fmt.Errorf("stage %s duration must be >= 0", stage.Name)
		}
		if stage.Result == "" {
			return fmt.Errorf("stage %s result is required", stage.Name)
		}
	}
	return nil
}

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
