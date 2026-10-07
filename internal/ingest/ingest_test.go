package ingest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nibinrj/PipelineIQ/internal/db"
	"github.com/nibinrj/PipelineIQ/internal/surefire"
)

func TestApplyIsIdempotent(t *testing.T) {
	dsn := os.Getenv("PIPELINEIQ_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("PIPELINEIQ_TEST_DATABASE_URL is unset and Docker is not used by this test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}

	svc := New(pool)
	job := "idempotent-job"
	report := Report{
		Repository:  "nibinrj/pipelineiq-lab",
		JobName:     job,
		BuildNumber: 7,
		Branch:      "main",
		CommitSHA:   "abc123",
		Result:      "FAILURE",
		AgentName:   "agent",
		LogTail:     "first tail",
		Stages: []Stage{{
			Name: "Build and test", DurationMs: 10, Result: "FAILURE",
		}},
		Tests: []Test{{
			Module: "core",
			Case: surefire.Case{
				ClassName: "io.pipelineiq.lab.core.CoreTest", MethodName: "adds",
				Outcome: surefire.Failed, DurationMs: 4, FailureType: "java.lang.AssertionError",
			},
		}},
	}
	first, err := svc.Apply(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	report.Result = "SUCCESS"
	report.LogTail = "second tail"
	report.Stages[0].Result = "SUCCESS"
	report.Tests[0].Outcome = surefire.Passed
	second, err := svc.Apply(ctx, report)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("build id changed from %d to %d", first.ID, second.ID)
	}
	if second.Result != "SUCCESS" || second.LogTail != "second tail" {
		t.Fatalf("update did not stick: %+v", second)
	}
	if len(second.Stages) != 1 || len(second.Tests) != 1 {
		t.Fatalf("children = stages %d tests %d", len(second.Stages), len(second.Tests))
	}
	if second.Tests[0].Outcome != surefire.Passed || second.Tests[0].Stage != "BLOCKING" {
		t.Fatalf("test = %+v", second.Tests[0])
	}
	assertOne(t, ctx, pool, "SELECT count(*) FROM build WHERE job_name = $1 AND build_number = 7", job)
	assertOne(t, ctx, pool, "SELECT count(*) FROM stage_run WHERE build_id = $1", first.ID)
	assertOne(t, ctx, pool, "SELECT count(*) FROM test_run WHERE build_id = $1", first.ID)
}

func assertOne(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, arg any) {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, query, arg).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("%s count = %d, want 1", query, n)
	}
}
