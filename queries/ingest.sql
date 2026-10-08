-- name: GetOrCreateRepository :one
INSERT INTO repository (name)
VALUES ($1)
ON CONFLICT (name) DO UPDATE
SET name = repository.name
RETURNING id;

-- name: UpsertBuild :one
INSERT INTO build (
    repository_id,
    job_name,
    build_number,
    branch,
    pr_number,
    commit_sha,
    result,
    started_at,
    finished_at,
    duration_ms,
    agent_name,
    agent_lifecycle,
    log_tail
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 'LOCAL', $12
)
ON CONFLICT (job_name, build_number) DO UPDATE
SET repository_id = EXCLUDED.repository_id,
    branch = EXCLUDED.branch,
    pr_number = EXCLUDED.pr_number,
    commit_sha = EXCLUDED.commit_sha,
    result = EXCLUDED.result,
    started_at = EXCLUDED.started_at,
    finished_at = EXCLUDED.finished_at,
    duration_ms = EXCLUDED.duration_ms,
    agent_name = EXCLUDED.agent_name,
    agent_lifecycle = EXCLUDED.agent_lifecycle,
    log_tail = EXCLUDED.log_tail,
    updated_at = now()
RETURNING id;

-- name: DeleteStageRunsByBuild :exec
DELETE FROM stage_run WHERE build_id = $1;

-- name: DeleteTestRunsByBuild :exec
DELETE FROM test_run WHERE build_id = $1;

-- name: InsertStageRun :exec
INSERT INTO stage_run (build_id, name, started_at, duration_ms, result)
VALUES ($1, $2, $3, $4, $5);

-- name: UpsertTestCase :one
INSERT INTO test_case (repository_id, module, class_name, method_name)
VALUES ($1, $2, $3, $4)
ON CONFLICT (repository_id, class_name, method_name) DO UPDATE
SET module = EXCLUDED.module
RETURNING id;

-- name: InsertTestRun :exec
INSERT INTO test_run (
    build_id,
    test_case_id,
    outcome,
    duration_ms,
    rerun_failures,
    failure_type,
    failure_hash,
    stage
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: GetBuildByID :one
SELECT
    b.id,
    r.name AS repository_name,
    b.job_name,
    b.build_number,
    b.branch,
    b.pr_number,
    b.commit_sha,
    b.result,
    b.started_at,
    b.finished_at,
    b.duration_ms,
    b.agent_name,
    b.agent_lifecycle,
    b.log_tail,
    b.infra_failure,
    b.infra_reason
FROM build b
JOIN repository r ON r.id = b.repository_id
WHERE b.id = $1;

-- name: ListStageRunsByBuild :many
SELECT id, name, started_at, duration_ms, result
FROM stage_run
WHERE build_id = $1
ORDER BY id;

-- name: ListTestRunsByBuild :many
SELECT
    tr.id,
    tc.module,
    tc.class_name,
    tc.method_name,
    tr.outcome,
    tr.duration_ms,
    tr.rerun_failures,
    tr.failure_type,
    tr.failure_hash,
    tr.stage
FROM test_run tr
JOIN test_case tc ON tc.id = tr.test_case_id
WHERE tr.build_id = $1
ORDER BY tc.class_name, tc.method_name, tr.id;
