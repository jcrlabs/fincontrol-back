package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/jonathanCaamano/fincontrol-back/internal/middleware"
	"github.com/shopspring/decimal"
)

type dashboardAccountRepo interface {
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	GetBalance(ctx context.Context, accountID, userID uuid.UUID) (decimal.Decimal, error)
}

type dashboardBudgetRepo interface {
	ListWithProgress(ctx context.Context, userID uuid.UUID, month time.Time) ([]domain.BudgetProgress, error)
}

type dashboardLedgerRepo interface {
	ListJournalEntries(ctx context.Context, filters app.JournalFilters) ([]domain.JournalEntry, int, error)
}

// DashboardHandler handles GET /api/v1/dashboard.
type DashboardHandler struct {
	report  reportRepository
	budgets dashboardBudgetRepo
	ledger  dashboardLedgerRepo
	accounts dashboardAccountRepo
}

func NewDashboardHandler(
	report reportRepository,
	budgets dashboardBudgetRepo,
	ledger dashboardLedgerRepo,
	accounts dashboardAccountRepo,
) *DashboardHandler {
	return &DashboardHandler{report: report, budgets: budgets, ledger: ledger, accounts: accounts}
}

func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	ctx := r.Context()
	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	// P&L current month
	pnl, err := h.report.GetProfitAndLoss(ctx, userID, monthStart, now)
	if err != nil {
		writeError(w, err)
		return
	}

	// Budget alerts (>= threshold)
	budgetProgress, err := h.budgets.ListWithProgress(ctx, userID, monthStart)
	if err != nil {
		writeError(w, err)
		return
	}
	var alerts []map[string]any
	for _, bp := range budgetProgress {
		if bp.IsAlert {
			alerts = append(alerts, map[string]any{
				"budget_id":   bp.ID.String(),
				"category_id": bp.CategoryID.String(),
				"amount":      bp.Amount.String(),
				"spent":       bp.Spent.String(),
				"percentage":  bp.Percentage.StringFixed(2),
				"is_exceeded": bp.IsExceeded,
			})
		}
	}

	// Recent transactions (last 10)
	journals, _, err := h.ledger.ListJournalEntries(ctx, app.JournalFilters{
		UserID: userID, Page: 1, PageSize: 10,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	recentTxs := make([]map[string]any, len(journals))
	for i, j := range journals {
		recentTxs[i] = map[string]any{
			"id":          j.ID.String(),
			"description": j.Description,
			"date":        j.Date.Format("2006-01-02"),
			"is_reversal": j.IsReversal,
		}
	}

	// Accounts with balances
	accs, err := h.accounts.ListByUser(ctx, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	accountList := make([]map[string]any, len(accs))
	for i, a := range accs {
		balance, _ := h.accounts.GetBalance(ctx, a.ID, userID)
		accountList[i] = map[string]any{
			"id":       a.ID.String(),
			"name":     a.Name,
			"type":     string(a.Type),
			"currency": a.Currency,
			"balance":  balance.String(),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"period":              map[string]string{"from": monthStart.Format("2006-01-02"), "to": now.Format("2006-01-02")},
		"income":              pnl.Income.String(),
		"expenses":            pnl.Expenses.String(),
		"net":                 pnl.Net.String(),
		"budget_alerts":       alerts,
		"recent_transactions": recentTxs,
		"accounts":            accountList,
	})
}
