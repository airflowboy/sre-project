package main

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// store is a thin wrapper over pgxpool with the schema bootstrap baked in.
type store struct {
	pool *pgxpool.Pool
}

func newStore(ctx context.Context, dsn string) (*store, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("new pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &store{pool: pool}, nil
}

func (s *store) Close() { s.pool.Close() }

// ensureSchema is the simplest possible migration: idempotent CREATE statements
// run at boot. Good enough for the learning phase; production should use a
// real migration tool with versioned files and locking.
func (s *store) ensureSchema(ctx context.Context) error {
	const ddl = `
		CREATE TABLE IF NOT EXISTS issued (
			issue_id         TEXT PRIMARY KEY,
			event_id         TEXT NOT NULL,
			user_id          TEXT NOT NULL,
			idempotency_key  TEXT NOT NULL,
			issued_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_issued_event_user ON issued(event_id, user_id);
	`
	_, err := s.pool.Exec(ctx, ddl)
	return err
}

// insertIssued is the consumer's hot path. ON CONFLICT DO NOTHING makes the
// operation idempotent: a re-delivered Kafka message yields no new row.
func (s *store) insertIssued(ctx context.Context, evt issuanceEvent) error {
	const q = `
		INSERT INTO issued (issue_id, event_id, user_id, idempotency_key, issued_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (issue_id) DO NOTHING
	`
	_, err := s.pool.Exec(ctx, q, evt.IssueID, evt.EventID, evt.UserID, evt.IdempotencyKey, evt.IssuedAt)
	return err
}
