package app

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// BudgetService handles budget management and progress evaluation.
type BudgetService struct {
	repo BudgetRepository
}

func NewBudgetService(repo BudgetRepository) *BudgetService {
	return &BudgetService{repo: repo}
}

type CreateBudgetInput struct {
	UserID            uuid.UUID
	CategoryID        uuid.UUID
	Month             time.Time
	Amount            decimal.Decimal
	AlertThresholdPct int
}

func (s *BudgetService) Create(ctx context.Context, input CreateBudgetInput) (domain.Budget, error) {
	if input.Amount.LessThanOrEqual(decimal.Zero) {
		return domain.Budget{}, fmt.Errorf("%w: amount must be positive", domain.ErrInvalidInput)
	}
	threshold := input.AlertThresholdPct
	if threshold == 0 {
		threshold = 80
	}
	// Normalize to first day of month
	month := time.Date(input.Month.Year(), input.Month.Month(), 1, 0, 0, 0, 0, time.UTC)

	budget := domain.Budget{
		ID:                uuid.New(),
		UserID:            input.UserID,
		CategoryID:        input.CategoryID,
		Month:             month,
		Amount:            input.Amount,
		AlertThresholdPct: threshold,
	}
	return s.repo.Create(ctx, budget)
}

func (s *BudgetService) ListWithProgress(ctx context.Context, userID uuid.UUID, month time.Time) ([]domain.BudgetProgress, error) {
	m := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	return s.repo.ListWithProgress(ctx, userID, m)
}

type UpdateBudgetInput struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Amount            decimal.Decimal
	AlertThresholdPct int
}

func (s *BudgetService) Update(ctx context.Context, input UpdateBudgetInput) (domain.Budget, error) {
	existing, err := s.repo.GetByID(ctx, input.ID, input.UserID)
	if err != nil {
		return domain.Budget{}, err
	}
	if input.Amount.GreaterThan(decimal.Zero) {
		existing.Amount = input.Amount
	}
	if input.AlertThresholdPct > 0 {
		existing.AlertThresholdPct = input.AlertThresholdPct
	}
	return s.repo.Update(ctx, existing)
}
