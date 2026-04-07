package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// BudgetRepo implements app.BudgetRepository using PostgreSQL.
type BudgetRepo struct {
	pool *pgxpool.Pool
}

func NewBudgetRepo(pool *pgxpool.Pool) *BudgetRepo {
	return &BudgetRepo{pool: pool}
}

func (r *BudgetRepo) Create(ctx context.Context, b domain.Budget) (domain.Budget, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO budgets (id, user_id, category_id, month, amount, alert_threshold_pct)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, category_id, month, amount, alert_threshold_pct
	`, b.ID, b.UserID, b.CategoryID, b.Month, b.Amount, b.AlertThresholdPct,
	).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Month, &b.Amount, &b.AlertThresholdPct)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.Budget{}, fmt.Errorf("%w: budget already exists for this category and month", domain.ErrConflict)
		}
		return domain.Budget{}, fmt.Errorf("create budget: %w", err)
	}
	return b, nil
}

func (r *BudgetRepo) ListWithProgress(ctx context.Context, userID uuid.UUID, month time.Time) ([]domain.BudgetProgress, error) {
	// Calculate spending per category for the given month using entries + journal_entries
	rows, err := r.pool.Query(ctx, `
		SELECT
			b.id, b.user_id, b.category_id, b.month, b.amount, b.alert_threshold_pct,
			COALESCE(SUM(e.amount) FILTER (WHERE e.amount > 0), 0) AS spent
		FROM budgets b
		LEFT JOIN journal_entries je ON
			je.user_id = b.user_id
			AND je.category_id = b.category_id
			AND date_trunc('month', je.date) = date_trunc('month', b.month::date)
		LEFT JOIN entries e ON e.journal_entry_id = je.id AND e.amount > 0
		WHERE b.user_id = $1
		GROUP BY b.id
		ORDER BY b.month DESC, b.category_id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list budgets with progress: %w", err)
	}
	defer rows.Close()

	var results []domain.BudgetProgress
	for rows.Next() {
		var bp domain.BudgetProgress
		var spent decimal.Decimal
		if err := rows.Scan(
			&bp.ID, &bp.UserID, &bp.CategoryID, &bp.Month, &bp.Amount, &bp.AlertThresholdPct,
			&spent,
		); err != nil {
			return nil, fmt.Errorf("scan budget progress: %w", err)
		}
		bp.Spent = spent
		if bp.Amount.GreaterThan(decimal.Zero) {
			bp.Percentage = spent.Div(bp.Amount).Mul(decimal.NewFromInt(100))
		}
		threshold := decimal.NewFromInt(int64(bp.AlertThresholdPct))
		bp.IsAlert = bp.Percentage.GreaterThanOrEqual(threshold)
		bp.IsExceeded = bp.Percentage.GreaterThanOrEqual(decimal.NewFromInt(100))
		results = append(results, bp)
	}
	return results, rows.Err()
}

func (r *BudgetRepo) GetByID(ctx context.Context, id, userID uuid.UUID) (domain.Budget, error) {
	var b domain.Budget
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, category_id, month, amount, alert_threshold_pct
		FROM budgets WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Month, &b.Amount, &b.AlertThresholdPct)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Budget{}, domain.ErrNotFound
		}
		return domain.Budget{}, fmt.Errorf("get budget: %w", err)
	}
	return b, nil
}

func (r *BudgetRepo) Update(ctx context.Context, b domain.Budget) (domain.Budget, error) {
	err := r.pool.QueryRow(ctx, `
		UPDATE budgets SET amount = $1, alert_threshold_pct = $2
		WHERE id = $3 AND user_id = $4
		RETURNING id, user_id, category_id, month, amount, alert_threshold_pct
	`, b.Amount, b.AlertThresholdPct, b.ID, b.UserID,
	).Scan(&b.ID, &b.UserID, &b.CategoryID, &b.Month, &b.Amount, &b.AlertThresholdPct)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Budget{}, domain.ErrNotFound
		}
		return domain.Budget{}, fmt.Errorf("update budget: %w", err)
	}
	return b, nil
}
