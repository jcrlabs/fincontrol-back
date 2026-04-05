package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
)

// CategoryRepo implements app.CategoryRepository using PostgreSQL.
type CategoryRepo struct {
	pool *pgxpool.Pool
}

func NewCategoryRepo(pool *pgxpool.Pool) *CategoryRepo {
	return &CategoryRepo{pool: pool}
}

func (r *CategoryRepo) Create(ctx context.Context, cat domain.Category) (domain.Category, error) {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO categories (id, user_id, name, parent_id, icon, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, name, parent_id, icon, created_at
	`, cat.ID, cat.UserID, cat.Name, cat.ParentID, cat.Icon, cat.CreatedAt,
	).Scan(&cat.ID, &cat.UserID, &cat.Name, &cat.ParentID, &cat.Icon, &cat.CreatedAt)
	if err != nil {
		return domain.Category{}, fmt.Errorf("create category: %w", err)
	}
	return cat, nil
}

func (r *CategoryRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, name, parent_id, icon, created_at
		FROM categories WHERE user_id = $1
		ORDER BY name
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var cats []domain.Category
	for rows.Next() {
		var c domain.Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.ParentID, &c.Icon, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

func (r *CategoryRepo) Update(ctx context.Context, cat domain.Category) (domain.Category, error) {
	err := r.pool.QueryRow(ctx, `
		UPDATE categories SET name = $1, parent_id = $2, icon = $3
		WHERE id = $4 AND user_id = $5
		RETURNING id, user_id, name, parent_id, icon, created_at
	`, cat.Name, cat.ParentID, cat.Icon, cat.ID, cat.UserID,
	).Scan(&cat.ID, &cat.UserID, &cat.Name, &cat.ParentID, &cat.Icon, &cat.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Category{}, domain.ErrNotFound
		}
		return domain.Category{}, fmt.Errorf("update category: %w", err)
	}
	return cat, nil
}
