-- name: UpsertRepository :one
INSERT INTO repository (name, default_branch)
VALUES ($1, $2)
ON CONFLICT (name) DO UPDATE
SET default_branch = EXCLUDED.default_branch
RETURNING id, name, default_branch, created_at;

-- name: GetRepositoryByName :one
SELECT id, name, default_branch, created_at
FROM repository
WHERE name = $1;
