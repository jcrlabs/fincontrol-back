package app

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// AuthRepository defines persistence operations for authentication.
type AuthRepository interface {
	CreateUser(ctx context.Context, user domain.User) (domain.User, error)
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

// AccountRepository defines persistence operations for accounts.
type AccountRepository interface {
	Create(ctx context.Context, account domain.Account) (domain.Account, error)
	GetByID(ctx context.Context, id, userID uuid.UUID) (domain.Account, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	Update(ctx context.Context, account domain.Account) (domain.Account, error)
	GetBalance(ctx context.Context, accountID, userID uuid.UUID) (decimal.Decimal, error)
	// GetOrCreateUncategorized returns (creating if needed) the system "Sin categorizar"
	// expense account used as the default counterpart during CSV imports.
	GetOrCreateUncategorized(ctx context.Context, userID uuid.UUID, currency string) (domain.Account, error)
}

// LedgerRepository defines persistence for journal entries (double-entry ledger).
type LedgerRepository interface {
	CreateJournalEntry(ctx context.Context, input CreateTransactionInput) (domain.JournalEntry, error)
	ListJournalEntries(ctx context.Context, filters JournalFilters) ([]domain.JournalEntry, int, error)
	GetJournalEntry(ctx context.Context, id, userID uuid.UUID) (domain.JournalEntry, error)
	VoidJournalEntry(ctx context.Context, id, userID uuid.UUID) (domain.JournalEntry, error)
	ListEntriesByAccount(ctx context.Context, accountID, userID uuid.UUID, page, pageSize int) ([]domain.Entry, int, error)
}

// AuditRepository defines append-only audit log operations.
type AuditRepository interface {
	Log(ctx context.Context, entry domain.AuditEntry) error
}

// CategoryRepository defines persistence for hierarchical categories.
type CategoryRepository interface {
	Create(ctx context.Context, category domain.Category) (domain.Category, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Category, error)
	Update(ctx context.Context, category domain.Category) (domain.Category, error)
}

// BudgetRepository defines persistence for budgets.
type BudgetRepository interface {
	Create(ctx context.Context, budget domain.Budget) (domain.Budget, error)
	GetByID(ctx context.Context, id, userID uuid.UUID) (domain.Budget, error)
	ListWithProgress(ctx context.Context, userID uuid.UUID, month time.Time) ([]domain.BudgetProgress, error)
	Update(ctx context.Context, budget domain.Budget) (domain.Budget, error)
}

// ReportRepository provides aggregated financial queries.
type ReportRepository interface {
	GetProfitAndLoss(ctx context.Context, userID uuid.UUID, from, to time.Time) (ProfitAndLoss, error)
	GetBalanceSheet(ctx context.Context, userID uuid.UUID, asOf time.Time) (BalanceSheet, error)
	GetCashFlow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]CashFlowPeriod, error)
	GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]CategoryBreakdown, error)
}

// ScheduleRepository defines persistence for recurring transactions.
type ScheduleRepository interface {
	Create(ctx context.Context, s domain.ScheduledTransaction) (domain.ScheduledTransaction, error)
	ListDue(ctx context.Context, before time.Time) ([]domain.ScheduledTransaction, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.ScheduledTransaction, error)
	Update(ctx context.Context, s domain.ScheduledTransaction) (domain.ScheduledTransaction, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

// --- Input/Output types ---

type CreateTransactionInput struct {
	UserID      uuid.UUID
	Description string
	Date        time.Time
	CategoryID  *uuid.UUID
	Entries     []EntryInput
}

type EntryInput struct {
	AccountID uuid.UUID
	Amount    decimal.Decimal
	Currency  string
}

type JournalFilters struct {
	UserID     uuid.UUID
	DateFrom   *time.Time
	DateTo     *time.Time
	AccountID  *uuid.UUID
	CategoryID *uuid.UUID
	Page       int
	PageSize   int
}

// Report types

type PnLPeriod struct {
	Month    time.Time
	Income   decimal.Decimal
	Expenses decimal.Decimal
	Net      decimal.Decimal
}

type ProfitAndLoss struct {
	From          time.Time
	To            time.Time
	Periods       []PnLPeriod
	TotalIncome   decimal.Decimal
	TotalExpenses decimal.Decimal
	TotalNet      decimal.Decimal
}

type BalanceSheet struct {
	AsOf        time.Time
	Assets      []AccountBalance
	Liabilities []AccountBalance
	Equity      []AccountBalance
	NetWorth    decimal.Decimal
}

type AccountBalance struct {
	AccountID uuid.UUID
	Name      string
	Balance   decimal.Decimal
}

type CashFlowPeriod struct {
	Month   time.Time
	Inflow  decimal.Decimal
	Outflow decimal.Decimal
	Net     decimal.Decimal
}

type CategoryBreakdown struct {
	CategoryID uuid.UUID
	Name       string
	Total      decimal.Decimal
	Percentage decimal.Decimal
	Children   []CategoryBreakdown
}
