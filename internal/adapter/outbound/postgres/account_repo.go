package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// AccountRepo implements app.AccountRepository using PostgreSQL.
type AccountRepo struct {
	pool *pgxpool.Pool
}

func NewAccountRepo(pool *pgxpool.Pool) *AccountRepo {
	return &AccountRepo{pool: pool}
}

func (r *AccountRepo) Create(ctx context.Context, account domain.Account) (domain.Account, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO accounts (id, user_id, name, type, currency, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, name, type, currency, is_active, created_at
	`, account.ID, account.UserID, account.Name, account.Type, account.Currency,
		account.IsActive, account.CreatedAt,
	).Scan(
		&account.ID, &account.UserID, &account.Name, &account.Type,
		&account.Currency, &account.IsActive, &account.CreatedAt,
	)
	if err != nil {
		return domain.Account{}, fmt.Errorf("create account: %w", err)
	}
	return account, nil
}

func (r *AccountRepo) GetByID(ctx context.Context, id, userID uuid.UUID) (domain.Account, error) {
	var a domain.Account
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, name, type, currency, is_active, created_at
		FROM accounts WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(
		&a.ID, &a.UserID, &a.Name, &a.Type, &a.Currency, &a.IsActive, &a.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrNotFound
		}
		return domain.Account{}, fmt.Errorf("get account: %w", err)
	}
	return a, nil
}

func (r *AccountRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Account, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, name, type, currency, is_active, created_at
		FROM accounts WHERE user_id = $1 AND is_active = true
		ORDER BY type, name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []domain.Account
	for rows.Next() {
		var a domain.Account
		if err := rows.Scan(&a.ID, &a.UserID, &a.Name, &a.Type, &a.Currency, &a.IsActive, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (r *AccountRepo) Update(ctx context.Context, account domain.Account) (domain.Account, error) {
	err := r.pool.QueryRow(ctx, `
		UPDATE accounts SET name = $1 WHERE id = $2 AND user_id = $3
		RETURNING id, user_id, name, type, currency, is_active, created_at
	`, account.Name, account.ID, account.UserID).Scan(
		&account.ID, &account.UserID, &account.Name, &account.Type,
		&account.Currency, &account.IsActive, &account.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Account{}, domain.ErrNotFound
		}
		return domain.Account{}, fmt.Errorf("update account: %w", err)
	}
	return account, nil
}

func (r *AccountRepo) GetBalance(ctx context.Context, accountID, userID uuid.UUID) (decimal.Decimal, error) {
	// Verify account ownership first
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1 AND user_id = $2)`,
		accountID, userID,
	).Scan(&exists)
	if err != nil {
		return decimal.Zero, fmt.Errorf("check account ownership: %w", err)
	}
	if !exists {
		return decimal.Zero, domain.ErrNotFound
	}

	var balance decimal.Decimal
	err = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(e.amount), 0)
		FROM entries e
		JOIN journal_entries je ON e.journal_entry_id = je.id
		WHERE e.account_id = $1 AND je.user_id = $2
	`, accountID, userID).Scan(&balance)
	if err != nil {
		return decimal.Zero, fmt.Errorf("get balance: %w", err)
	}
	return balance, nil
}
