// Hand-written to match queries/quarantine.sql. sqlc was not run in this
// environment. Keep the SQL here in sync with that file.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const setBuildInfra = `
UPDATE build
SET infra_failure = $2,
    infra_reason = $3,
    updated_at = now()
WHERE id = $1
`

func (q *Queries) SetBuildInfra(ctx context.Context, id int64, failure bool, reason *string) error {
	_, err := q.db.Exec(ctx, setBuildInfra, id, failure, reason)
	return err
}

const getRepositoryDefaultBranch = `
SELECT default_branch FROM repository WHERE id = $1
`

func (q *Queries) GetRepositoryDefaultBranch(ctx context.Context, id int64) (string, error) {
	row := q.db.QueryRow(ctx, getRepositoryDefaultBranch, id)
	var branch string
	err := row.Scan(&branch)
	return branch, err
}

const getRepositoryByName = `
SELECT id, default_branch FROM repository WHERE name = $1
`

type GetRepositoryByNameRow struct {
	ID            int64
	DefaultBranch string
}

func (q *Queries) GetRepositoryByName(ctx context.Context, name string) (GetRepositoryByNameRow, error) {
	row := q.db.QueryRow(ctx, getRepositoryByName, name)
	var i GetRepositoryByNameRow
	err := row.Scan(&i.ID, &i.DefaultBranch)
	return i, err
}

const listBuildIDsForRules = `
SELECT id, infra_failure
FROM build
WHERE repository_id = $1
ORDER BY id
`

type ListBuildIDsForRulesRow struct {
	ID           int64
	InfraFailure bool
}

func (q *Queries) ListBuildIDsForRules(ctx context.Context, repositoryID int64) ([]ListBuildIDsForRulesRow, error) {
	rows, err := q.db.Query(ctx, listBuildIDsForRules, repositoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListBuildIDsForRulesRow{}
	for rows.Next() {
		var i ListBuildIDsForRulesRow
		if err := rows.Scan(&i.ID, &i.InfraFailure); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listTestRunsForRules = `
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
ORDER BY tr.test_case_id, b.id, tr.id
`

type ListTestRunsForRulesRow struct {
	TestCaseID   int64
	BuildID      int64
	Outcome      string
	Stage        string
	CommitSha    string
	Branch       string
	InfraFailure bool
}

func (q *Queries) ListTestRunsForRules(ctx context.Context, repositoryID int64) ([]ListTestRunsForRulesRow, error) {
	rows, err := q.db.Query(ctx, listTestRunsForRules, repositoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListTestRunsForRulesRow{}
	for rows.Next() {
		var i ListTestRunsForRulesRow
		if err := rows.Scan(
			&i.TestCaseID,
			&i.BuildID,
			&i.Outcome,
			&i.Stage,
			&i.CommitSha,
			&i.Branch,
			&i.InfraFailure,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const countTestCases = `
SELECT count(*)::bigint FROM test_case WHERE repository_id = $1
`

func (q *Queries) CountTestCases(ctx context.Context, repositoryID int64) (int64, error) {
	row := q.db.QueryRow(ctx, countTestCases, repositoryID)
	var n int64
	err := row.Scan(&n)
	return n, err
}

const listActiveQuarantineByRepo = `
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
ORDER BY tc.class_name, tc.method_name, q.id
`

type ListActiveQuarantineByRepoRow struct {
	ID                 int64
	TestCaseID         int64
	ReasonRule         string
	Evidence           string
	QuarantinedAt      time.Time
	ConsecutivePasses  int32
	Manual             bool
	State              string
	ClassName          string
	MethodName         string
}

func (q *Queries) ListActiveQuarantineByRepo(ctx context.Context, repositoryID int64) ([]ListActiveQuarantineByRepoRow, error) {
	rows, err := q.db.Query(ctx, listActiveQuarantineByRepo, repositoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []ListActiveQuarantineByRepoRow{}
	for rows.Next() {
		var i ListActiveQuarantineByRepoRow
		if err := rows.Scan(
			&i.ID,
			&i.TestCaseID,
			&i.ReasonRule,
			&i.Evidence,
			&i.QuarantinedAt,
			&i.ConsecutivePasses,
			&i.Manual,
			&i.State,
			&i.ClassName,
			&i.MethodName,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const insertQuarantine = `
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
RETURNING id, quarantined_at
`

type InsertQuarantineRow struct {
	ID            int64
	QuarantinedAt time.Time
}

func (q *Queries) InsertQuarantine(ctx context.Context, testCaseID int64, reason, evidence string, manual bool) (InsertQuarantineRow, error) {
	row := q.db.QueryRow(ctx, insertQuarantine, testCaseID, reason, evidence, manual)
	var i InsertQuarantineRow
	err := row.Scan(&i.ID, &i.QuarantinedAt)
	return i, err
}

const updateQuarantinePasses = `
UPDATE quarantine
SET consecutive_passes = $2
WHERE id = $1
  AND state = 'QUARANTINED'
`

func (q *Queries) UpdateQuarantinePasses(ctx context.Context, id int64, passes int32) error {
	_, err := q.db.Exec(ctx, updateQuarantinePasses, id, passes)
	return err
}

const releaseQuarantine = `
UPDATE quarantine
SET state = 'RELEASED',
    released_at = now(),
    consecutive_passes = $2
WHERE id = $1
  AND state = 'QUARANTINED'
`

func (q *Queries) ReleaseQuarantine(ctx context.Context, id int64, passes int32) error {
	_, err := q.db.Exec(ctx, releaseQuarantine, id, passes)
	return err
}

const insertFlakyAlert = `
INSERT INTO alert (type, build_id, repository_id, payload)
VALUES ('FLAKY', $1, $2, $3::jsonb)
`

func (q *Queries) InsertFlakyAlert(ctx context.Context, buildID, repositoryID int64, payload string) error {
	_, err := q.db.Exec(ctx, insertFlakyAlert, buildID, repositoryID, payload)
	return err
}

const getTestCaseByName = `
SELECT id
FROM test_case
WHERE repository_id = $1
  AND class_name = $2
  AND method_name = $3
`

func (q *Queries) GetTestCaseByName(ctx context.Context, repositoryID int64, className, methodName string) (int64, error) {
	row := q.db.QueryRow(ctx, getTestCaseByName, repositoryID, className, methodName)
	var id int64
	err := row.Scan(&id)
	return id, err
}

const maxBuildID = `
SELECT COALESCE(MAX(id), 0)::bigint FROM build WHERE repository_id = $1
`

func (q *Queries) MaxBuildID(ctx context.Context, repositoryID int64) (int64, error) {
	row := q.db.QueryRow(ctx, maxBuildID, repositoryID)
	var id int64
	err := row.Scan(&id)
	return id, err
}

// ErrNoRows is the pgx empty-result error, re-exported so callers outside store
// do not need to know which driver said it.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
