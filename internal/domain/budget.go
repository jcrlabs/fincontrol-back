package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Budget tracks spending limits per category per month.
type Budget struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	CategoryID        uuid.UUID
	Month             time.Time // first day of the month
	Amount            decimal.Decimal
	AlertThresholdPct int // default 80
}

// BudgetProgress is a calculated view of a budget with current spending.
type BudgetProgress struct {
	Budget
	Spent      decimal.Decimal
	Percentage decimal.Decimal
	IsAlert    bool // >= alert threshold
	IsExceeded bool // >= 100%
}
