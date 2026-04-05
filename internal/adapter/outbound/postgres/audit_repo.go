package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
)

// AuditRepo implements app.AuditRepository — append-only.
type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

func (r *AuditRepo) Log(ctx context.Context, entry domain.AuditEntry) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_log (user_id, action, entity_type, entity_id, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, entry.UserID, entry.Action, entry.EntityType, entry.EntityID, entry.Payload)
	if err != nil {
		return fmt.Errorf("audit log: %w", err)
	}
	return nil
}
