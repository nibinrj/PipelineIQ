-- name: GetBuildByJobNumber :one
SELECT
    id,
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
    agent_instance_type,
    agent_lifecycle,
    infra_failure,
    infra_reason,
    cost_usd,
    created_at,
    updated_at
FROM build
WHERE job_name = $1 AND build_number = $2;
