package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// AccountType follows GAAP classification.
type AccountType string

const (
	AccountTypeAsset     AccountType = "asset"
	AccountTypeLiability AccountType = "liability"
	AccountTypeEquity    AccountType = "equity"
	AccountTypeIncome    AccountType = "income"
	AccountTypeExpense   AccountType = "expense"
)

func (t AccountType) IsValid() bool {
	switch t {
	case AccountTypeAsset, AccountTypeLiability, AccountTypeEquity, AccountTypeIncome, AccountTypeExpense:
		return true
	}
	return false
}

// Account is the core entity for tracking financial resources.
type Account struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	Type      AccountType
	Currency  string
	IsActive  bool
	Balance   decimal.Decimal // calculated field — not stored, derived from entries
	CreatedAt time.Time
}
