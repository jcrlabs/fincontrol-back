package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ImportRepo implements app.ImportDedupRepository using PostgreSQL.
type ImportRepo struct {
	pool *pgxpool.Pool
}

func NewImportRepo(pool *pgxpool.Pool) *ImportRepo {
	return &ImportRepo{pool: pool}
}

func (r *ImportRepo) IsDuplicate(ctx context.Context, userID uuid.UUID, hash string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM import_dedup WHERE user_id = $1 AND row_hash = $2)`,
		userID, hash,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check import dedup: %w", err)
	}
	return exists, nil
}

func (r *ImportRepo) RecordImport(ctx context.Context, userID uuid.UUID, hash string, journalID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO import_dedup (user_id, row_hash, journal_entry_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		userID, hash, journalID,
	)
	if err != nil {
		return fmt.Errorf("record import dedup: %w", err)
	}
	return nil
}
