package quarantine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nibinrj/PipelineIQ/internal/store"
)

// ErrNotFound is a missing repository or an inactive quarantine.
var ErrNotFound = errors.New("not found")

// Item is one quarantine row returned by the API.
type Item struct {
	ID                int64           `json:"id"`
	ClassName         string          `json:"class_name"`
	MethodName        string          `json:"method_name"`
	ReasonRule        string          `json:"reason_rule"`
	Evidence          json.RawMessage `json:"evidence,omitempty"`
	QuarantinedAt     time.Time       `json:"quarantined_at"`
	ConsecutivePasses int32           `json:"consecutive_passes"`
	Manual            bool            `json:"manual"`
	State             string          `json:"state"`
}

// List is the active quarantine for one repository.
type List struct {
	Repository string `json:"repository"`
	Active     []Item `json:"active"`
}

// Service reads and writes quarantine rows. Ingest calls AfterIngest on the
// same transaction; these methods open their own.
type Service struct {
	pool *pgxpool.Pool
}

// New binds the service to the process pool.
func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// List returns the active rows. An unknown repository is an empty list.
func (s *Service) List(ctx context.Context, repo string) (List, error) {
	q := store.New(s.pool)
	out := List{Repository: repo, Active: []Item{}}
	row, err := q.GetRepositoryByName(ctx, repo)
	if err != nil {
		if store.IsNoRows(err) {
			return out, nil
		}
		return List{}, fmt.Errorf("repository: %w", err)
	}
	rows, err := q.ListActiveQuarantineByRepo(ctx, row.ID)
	if err != nil {
		return List{}, fmt.Errorf("list: %w", err)
	}
	for _, one := range rows {
		out.Active = append(out.Active, itemFrom(one))
	}
	return out, nil
}

// ManualQuarantine creates an active row. It is allowed over the safety cap.
// An existing active row is returned unchanged.
func (s *Service) ManualQuarantine(ctx context.Context, repo, className, methodName string) (Item, error) {
	if repo == "" || className == "" || methodName == "" {
		return Item{}, fmt.Errorf("repository, class, and method are required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Item{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)
	repoID, err := q.GetOrCreateRepository(ctx, repo)
	if err != nil {
		return Item{}, fmt.Errorf("repository: %w", err)
	}
	caseID, err := q.UpsertTestCase(ctx, store.UpsertTestCaseParams{
		RepositoryID: repoID,
		Module:       "",
		ClassName:    className,
		MethodName:   methodName,
	})
	if err != nil {
		return Item{}, fmt.Errorf("test case: %w", err)
	}
	if existing, ok, err := findActive(ctx, q, repoID, className, methodName); err != nil {
		return Item{}, err
	} else if ok {
		if err := tx.Commit(ctx); err != nil {
			return Item{}, fmt.Errorf("commit: %w", err)
		}
		return existing, nil
	}
	since, err := q.MaxBuildID(ctx, repoID)
	if err != nil {
		return Item{}, fmt.Errorf("latest build: %w", err)
	}
	evidence, err := json.Marshal(map[string]any{"rule": "MANUAL", "since_build_id": since})
	if err != nil {
		return Item{}, err
	}
	inserted, err := q.InsertQuarantine(ctx, caseID, "MANUAL", string(evidence), true)
	if err != nil {
		return Item{}, fmt.Errorf("insert: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Item{}, fmt.Errorf("commit: %w", err)
	}
	return Item{
		ID:            inserted.ID,
		ClassName:     className,
		MethodName:    methodName,
		ReasonRule:    "MANUAL",
		Evidence:      evidence,
		QuarantinedAt: inserted.QuarantinedAt,
		Manual:        true,
		State:         "QUARANTINED",
	}, nil
}

// ManualRelease marks the active row released. A missing row is ErrNotFound.
func (s *Service) ManualRelease(ctx context.Context, repo, className, methodName string) (Item, error) {
	if repo == "" || className == "" || methodName == "" {
		return Item{}, fmt.Errorf("repository, class, and method are required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Item{}, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := store.New(tx)
	repoRow, err := q.GetRepositoryByName(ctx, repo)
	if err != nil {
		if store.IsNoRows(err) {
			return Item{}, ErrNotFound
		}
		return Item{}, fmt.Errorf("repository: %w", err)
	}
	existing, ok, err := findActive(ctx, q, repoRow.ID, className, methodName)
	if err != nil {
		return Item{}, err
	}
	if !ok {
		return Item{}, ErrNotFound
	}
	if err := q.ReleaseQuarantine(ctx, existing.ID, existing.ConsecutivePasses); err != nil {
		return Item{}, fmt.Errorf("release: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Item{}, fmt.Errorf("commit: %w", err)
	}
	existing.State = "RELEASED"
	return existing, nil
}

func findActive(ctx context.Context, q *store.Queries, repoID int64, className, methodName string) (Item, bool, error) {
	rows, err := q.ListActiveQuarantineByRepo(ctx, repoID)
	if err != nil {
		return Item{}, false, fmt.Errorf("list: %w", err)
	}
	for _, row := range rows {
		if row.ClassName == className && row.MethodName == methodName {
			return itemFrom(row), true, nil
		}
	}
	return Item{}, false, nil
}

func itemFrom(row store.ListActiveQuarantineByRepoRow) Item {
	item := Item{
		ID:                row.ID,
		ClassName:         row.ClassName,
		MethodName:        row.MethodName,
		ReasonRule:        row.ReasonRule,
		QuarantinedAt:     row.QuarantinedAt,
		ConsecutivePasses: row.ConsecutivePasses,
		Manual:            row.Manual,
		State:             row.State,
	}
	if row.Evidence != "" && row.Evidence != "null" {
		item.Evidence = json.RawMessage(row.Evidence)
	}
	return item
}

// IsNotFound reports a missing quarantine.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
