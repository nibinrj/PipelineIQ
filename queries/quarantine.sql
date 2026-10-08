-- name: SetBuildInfra :exec
UPDATE build
SET infra_failure = $2,
    infra_reason = $3,
    updated_at = now()
WHERE id = $1;

-- name: GetRepositoryDefaultBranch :one
SELECT default_branch FROM repository WHERE id = $1;

-- name: GetRepositoryByName :one
SELECT id, default_branch FROM repository WHERE name = $1;

-- name: ListBuildIDsForRules :many
SELECT id, infra_failure
FROM build
WHERE repository_id = $1
ORDER BY id;

-- name: ListTestRunsForRules :many
SELECT
    tr.test_case_id,
    tr.build_id,
    tr.outcome,
    tr.stage,
    b.commit_sha,
    b.branch,
    b.infra_failure
FROM test_run tr
JOIN build b ON b.id = tr.build_id
JOIN test_case tc ON tc.id = tr.test_case_id
WHERE tc.repository_id = $1
ORDER BY tr.test_case_id, b.id, tr.id;

-- name: CountTestCases :one
SELECT count(*)::bigint FROM test_case WHERE repository_id = $1;

-- name: ListActiveQuarantineByRepo :many
SELECT
    q.id,
    q.test_case_id,
    q.reason_rule,
    q.evidence::text AS evidence,
    q.quarantined_at,
    q.consecutive_passes,
    q.manual,
    q.state,
    tc.class_name,
    tc.method_name
FROM quarantine q
JOIN test_case tc ON tc.id = q.test_case_id
WHERE tc.repository_id = $1
  AND q.state = 'QUARANTINED'
ORDER BY tc.class_name, tc.method_name, q.id;

-- name: InsertQuarantine :one
INSERT INTO quarantine (
    test_case_id,
    state,
    reason_rule,
    evidence,
    quarantined_at,
    consecutive_passes,
    manual
) VALUES (
    $1, 'QUARANTINED', $2, $3::jsonb, now(), 0, $4
)
ON CONFLICT (test_case_id) WHERE state = 'QUARANTINED' DO NOTHING
RETURNING id, quarantined_at;

-- name: UpdateQuarantinePasses :exec
UPDATE quarantine
SET consecutive_passes = $2
WHERE id = $1
  AND state = 'QUARANTINED';

-- name: ReleaseQuarantine :exec
UPDATE quarantine
SET state = 'RELEASED',
    released_at = now(),
    consecutive_passes = $2
WHERE id = $1
  AND state = 'QUARANTINED';

-- name: InsertFlakyAlert :exec
INSERT INTO alert (type, build_id, repository_id, payload)
VALUES ('FLAKY', $1, $2, $3::jsonb);

-- name: GetTestCaseByName :one
SELECT id
FROM test_case
WHERE repository_id = $1
  AND class_name = $2
  AND method_name = $3;

-- name: MaxBuildID :one
SELECT COALESCE(MAX(id), 0)::bigint FROM build WHERE repository_id = $1;
