package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// planTables are the tables docs/plan.md requires the first migration to create.
// goose_db_version is goose's own bookkeeping table and is not part of the plan.
var planTables = map[string][]string{
	"repository": {"id", "name", "default_branch"},
	"build": {
		"id", "repository_id", "job_name", "build_number", "branch", "pr_number",
		"commit_sha", "result", "started_at", "finished_at", "duration_ms",
		"agent_name", "agent_instance_type", "agent_lifecycle", "infra_failure",
		"infra_reason", "cost_usd", "log_tail",
	},
	"stage_run":   {"id", "build_id", "name", "started_at", "duration_ms", "result"},
	"test_case":   {"id", "repository_id", "module", "class_name", "method_name"},
	"test_run":    {"id", "build_id", "test_case_id", "outcome", "duration_ms", "rerun_failures", "failure_type", "failure_hash", "stage"},
	"quarantine":  {"id", "test_case_id", "state", "reason_rule", "evidence", "quarantined_at", "released_at", "consecutive_passes", "manual"},
	"alert":       {"id", "type", "build_id", "repository_id", "payload", "created_at", "pr_comment_id"},
	"price":       {"id", "instance_type", "lifecycle", "region", "usd_per_hour", "effective_from", "source"},
	"infra_event": {"id", "instance_id", "event_type", "event_time", "raw"},
}

func TestMigrationsApplyAndTablesExist(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	dsn := os.Getenv("PIPELINEIQ_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = postgresFromContainer(t, ctx)
	}

	pool, err := Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}

	var pgVersion string
	if err := pool.QueryRow(ctx, "SELECT version()").Scan(&pgVersion); err != nil {
		t.Fatalf("postgres version: %v", err)
	}
	t.Logf("postgres version: %s", pgVersion)

	assertPlanTables(t, ctx, pool)
	assertJobBuildUnique(t, ctx, pool)
	assertOneActiveQuarantine(t, ctx, pool)
}

func postgresFromContainer(t *testing.T, ctx context.Context) string {
	t.Helper()
	if _, err := os.Stat("/var/run/docker.sock"); err != nil {
		t.Skip("PIPELINEIQ_TEST_DATABASE_URL is unset and /var/run/docker.sock is not available")
	}
	// Same tag as docker-compose.yml. Pin source: docker-library/official-images library/postgres.
	container, err := postgres.Run(ctx, "postgres:18.6",
		postgres.WithDatabase("pipelineiq"),
		postgres.WithUsername("pipelineiq"),
		postgres.WithPassword("pipelineiq"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres:18.6: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	return dsn
}

func assertPlanTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for table, columns := range planTables {
		rows, err := pool.Query(ctx, `
			SELECT column_name
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1
		`, table)
		if err != nil {
			t.Fatalf("columns %s: %v", table, err)
		}
		got := map[string]bool{}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				rows.Close()
				t.Fatalf("scan %s: %v", table, err)
			}
			got[name] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("rows %s: %v", table, err)
		}
		if len(got) == 0 {
			t.Errorf("table %s does not exist", table)
			continue
		}
		for _, column := range columns {
			if !got[column] {
				t.Errorf("table %s missing column %s", table, column)
			}
		}
	}
}

func assertJobBuildUnique(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var repoID int64
	err := pool.QueryRow(ctx, `
		INSERT INTO repository (name, default_branch)
		VALUES ($1, 'main')
		RETURNING id
	`, fmt.Sprintf("unique-job-build-%d", time.Now().UnixNano())).Scan(&repoID)
	if err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	// job_name is part of build_job_number_unique and is not scoped to
	// repository_id, so a reused database needs a fresh job name per run.
	jobName := fmt.Sprintf("folder/job-%d", time.Now().UnixNano())
	insert := `
		INSERT INTO build (repository_id, job_name, build_number, branch, commit_sha, result)
		VALUES ($1, $2, 7, 'main', 'abc123', 'SUCCESS')
	`
	if _, err := pool.Exec(ctx, insert, repoID, jobName); err != nil {
		t.Fatalf("first build insert: %v", err)
	}
	_, err = pool.Exec(ctx, insert, repoID, jobName)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("second insert err = %v, want unique_violation 23505", err)
	}
}

func assertOneActiveQuarantine(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var repoID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO repository (name) VALUES ($1) RETURNING id
	`, fmt.Sprintf("quarantine-cap-%d", time.Now().UnixNano())).Scan(&repoID); err != nil {
		t.Fatalf("insert repository: %v", err)
	}
	var caseID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO test_case (repository_id, class_name, method_name)
		VALUES ($1, 'lab.FlakyTest', 'flips')
		RETURNING id
	`, repoID).Scan(&caseID); err != nil {
		t.Fatalf("insert test case: %v", err)
	}
	insert := `
		INSERT INTO quarantine (test_case_id, state, reason_rule, quarantined_at)
		VALUES ($1, 'QUARANTINED', 'R1', now())
	`
	if _, err := pool.Exec(ctx, insert, caseID); err != nil {
		t.Fatalf("first quarantine: %v", err)
	}
	_, err := pool.Exec(ctx, insert, caseID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		t.Fatalf("second active quarantine err = %v, want unique_violation 23505", err)
	}
}
