package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
)

// ScheduleRepo implements app.ScheduleRepository using PostgreSQL.
type ScheduleRepo struct {
	pool *pgxpool.Pool
}

func NewScheduleRepo(pool *pgxpool.Pool) *ScheduleRepo {
	return &ScheduleRepo{pool: pool}
}

func (r *ScheduleRepo) Create(ctx context.Context, s domain.ScheduledTransaction) (domain.ScheduledTransaction, error) {
	entriesJSON, err := json.Marshal(s.TemplateEntries)
	if err != nil {
		return domain.ScheduledTransaction{}, fmt.Errorf("marshal template entries: %w", err)
	}
	err = r.pool.QueryRow(ctx, `
		INSERT INTO scheduled_transactions (id, user_id, description, frequency, next_run, is_active, template_entries, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, description, frequency, next_run, is_active, template_entries, created_at
	`, s.ID, s.UserID, s.Description, s.Frequency, s.NextRun, s.IsActive, entriesJSON, s.CreatedAt,
	).Scan(&s.ID, &s.UserID, &s.Description, &s.Frequency, &s.NextRun, &s.IsActive, &entriesJSON, &s.CreatedAt)
	if err != nil {
		return domain.ScheduledTransaction{}, fmt.Errorf("create scheduled: %w", err)
	}
	if err := json.Unmarshal(entriesJSON, &s.TemplateEntries); err != nil {
		return domain.ScheduledTransaction{}, fmt.Errorf("unmarshal template entries: %w", err)
	}
	return s, nil
}

func (r *ScheduleRepo) ListDue(ctx context.Context, before time.Time) ([]domain.ScheduledTransaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, description, frequency, next_run, is_active, template_entries, created_at
		FROM scheduled_transactions
		WHERE is_active = true AND next_run <= $1
		ORDER BY next_run
	`, before)
	if err != nil {
		return nil, fmt.Errorf("list due scheduled: %w", err)
	}
	defer rows.Close()
	return scanScheduledRows(rows)
}

func (r *ScheduleRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.ScheduledTransaction, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, description, frequency, next_run, is_active, template_entries, created_at
		FROM scheduled_transactions WHERE user_id = $1
		ORDER BY next_run
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list scheduled: %w", err)
	}
	defer rows.Close()
	return scanScheduledRows(rows)
}

func (r *ScheduleRepo) Update(ctx context.Context, s domain.ScheduledTransaction) (domain.ScheduledTransaction, error) {
	entriesJSON, err := json.Marshal(s.TemplateEntries)
	if err != nil {
		return domain.ScheduledTransaction{}, fmt.Errorf("marshal template entries: %w", err)
	}
	err = r.pool.QueryRow(ctx, `
		UPDATE scheduled_transactions
		SET frequency = $1, next_run = $2, is_active = $3, template_entries = $4
		WHERE id = $5 AND user_id = $6
		RETURNING id, user_id, description, frequency, next_run, is_active, template_entries, created_at
	`, s.Frequency, s.NextRun, s.IsActive, entriesJSON, s.ID, s.UserID,
	).Scan(&s.ID, &s.UserID, &s.Description, &s.Frequency, &s.NextRun, &s.IsActive, &entriesJSON, &s.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ScheduledTransaction{}, domain.ErrNotFound
		}
		return domain.ScheduledTransaction{}, fmt.Errorf("update scheduled: %w", err)
	}
	if err := json.Unmarshal(entriesJSON, &s.TemplateEntries); err != nil {
		return domain.ScheduledTransaction{}, fmt.Errorf("unmarshal template entries: %w", err)
	}
	return s, nil
}

func (r *ScheduleRepo) Delete(ctx context.Context, id, userID uuid.UUID) error {
	// Soft delete — set is_active = false
	tag, err := r.pool.Exec(ctx,
		`UPDATE scheduled_transactions SET is_active = false WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete scheduled: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanScheduledRows(rows pgx.Rows) ([]domain.ScheduledTransaction, error) {
	var results []domain.ScheduledTransaction
	for rows.Next() {
		var s domain.ScheduledTransaction
		var entriesJSON []byte
		if err := rows.Scan(&s.ID, &s.UserID, &s.Description, &s.Frequency, &s.NextRun,
			&s.IsActive, &entriesJSON, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan scheduled: %w", err)
		}
		if err := json.Unmarshal(entriesJSON, &s.TemplateEntries); err != nil {
			return nil, fmt.Errorf("unmarshal template entries: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}
