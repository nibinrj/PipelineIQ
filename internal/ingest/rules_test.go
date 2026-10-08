package ingest

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/nibinrj/PipelineIQ/internal/db"
	"github.com/nibinrj/PipelineIQ/internal/quarantine"
	"github.com/nibinrj/PipelineIQ/internal/surefire"
)

func TestSameCommitQuarantinesAndInfraDoesNot(t *testing.T) {
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
	qsvc := quarantine.New(pool)
	repo := "nibinrj/pipelineiq-rules-fixture"
	job := "p3-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	pass := reportFor(repo, job, 1, "abc", surefire.Passed, "")
	if _, err := svc.Apply(ctx, pass); err != nil {
		t.Fatal(err)
	}
	fail := reportFor(repo, job, 2, "abc", surefire.Failed, "")
	if _, err := svc.Apply(ctx, fail); err != nil {
		t.Fatal(err)
	}
	list, err := qsvc.List(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range list.Active {
		if item.ClassName == "io.pipelineiq.lab.flaky.RulesFixture" && item.ReasonRule == "R2" {
			found = true
		}
	}
	if !found {
		t.Fatalf("quarantine = %+v", list.Active)
	}

	infraJob := job + "-infra"
	if _, err := svc.Apply(ctx, reportFor(repo, infraJob, 1, "def", surefire.Passed, "")); err != nil {
		t.Fatal(err)
	}
	oom := reportFor(repo, infraJob, 2, "def", surefire.Failed, "container status: OOMKilled")
	oom.Tests[0].ClassName = "io.pipelineiq.lab.flaky.InfraOnlyTest"
	oom.Tests[0].MethodName = "failsOnDeadAgent"
	view, err := svc.Apply(ctx, oom)
	if err != nil {
		t.Fatal(err)
	}
	if !view.InfraFailure || view.InfraReason != "OUT_OF_MEMORY" {
		t.Fatalf("infra = %v %s", view.InfraFailure, view.InfraReason)
	}
	list, err = qsvc.List(ctx, repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range list.Active {
		if item.MethodName == "failsOnDeadAgent" {
			t.Fatalf("infra failure was quarantined: %+v", item)
		}
	}
}

func reportFor(repo, job string, number int32, commit, outcome, logTail string) Report {
	return Report{
		Repository:  repo,
		JobName:     job,
		BuildNumber: number,
		Branch:      "main",
		CommitSHA:   commit,
		Result:      "SUCCESS",
		LogTail:     logTail,
		Tests: []Test{{
			Module: "flaky-lab",
			Case: surefire.Case{
				ClassName:  "io.pipelineiq.lab.flaky.RulesFixture",
				MethodName: "passAndFail",
				Outcome:    outcome,
				DurationMs: 10,
			},
		}},
	}
}
