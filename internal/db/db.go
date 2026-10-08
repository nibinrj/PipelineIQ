// Package db opens Postgres, applies migrations, and wraps the sqlc queries
// the HTTP server needs. Later prompts add ingest on top of the same pool.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/nibinrj/PipelineIQ/internal/store"
	"github.com/nibinrj/PipelineIQ/migrations"
)

// Connect opens a pool and pings it. It retries because compose can start the
// server in the same second Postgres begins accepting connections.
// MaxConns is 4 so the pool stays small under the 64 MB container limit.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 4
	cfg.MinConns = 0
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	var last error
	backoff := 200 * time.Millisecond
	for attempt := 1; attempt <= 10; attempt++ {
		pool, openErr := pgxpool.NewWithConfig(ctx, cfg)
		if openErr == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			openErr = pool.Ping(pingCtx)
			cancel()
			if openErr == nil {
				return pool, nil
			}
			pool.Close()
		}
		last = openErr
		if attempt == 10 {
			break
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("connect database: %w", ctx.Err())
		case <-timer.C:
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
	}
	return nil, fmt.Errorf("connect database: %w", last)
}

// Migrate applies embedded goose migrations. Running it twice is a no-op.
// goose.SetBaseFS and goose.SetDialect are process-wide; this service has one
// schema, so that is the same shape as a static holder in Java.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }() // does not close the pool; pgx documents that

	// goose's default logger writes plain text to stdout. The service logs JSON.
	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, sqlDB, "."); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Store is the small surface the HTTP server uses. sqlc generated the queries.
type Store struct {
	q *store.Queries
}

// NewStore binds sqlc queries to the pool. pgxpool.Pool implements the DBTX
// interface sqlc generated (Exec, Query, QueryRow).
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{q: store.New(pool)}
}

// Ping is the readiness check. It runs the generated SELECT 1, so a schema
// or permission problem fails the probe instead of only a TCP connect.
func (s *Store) Ping(ctx context.Context) error {
	ok, err := s.q.Ping(ctx)
	if err != nil {
		return fmt.Errorf("ping: %w", err)
	}
	if ok != 1 {
		return fmt.Errorf("ping: unexpected result %d", ok)
	}
	return nil
}
