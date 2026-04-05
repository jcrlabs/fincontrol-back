package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/middleware"
)

type reportRepository interface {
	GetProfitAndLoss(ctx context.Context, userID uuid.UUID, from, to time.Time) (app.ProfitAndLoss, error)
	GetBalanceSheet(ctx context.Context, userID uuid.UUID, asOf time.Time) (app.BalanceSheet, error)
	GetCashFlow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]app.CashFlowPeriod, error)
	GetCategoryBreakdown(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]app.CategoryBreakdown, error)
}

// ReportHandler handles /api/v1/reports/* routes.
// Reports are pure queries — no service layer needed, repo is called directly.
type ReportHandler struct {
	repo reportRepository
}

func NewReportHandler(repo reportRepository) *ReportHandler {
	return &ReportHandler{repo: repo}
}

// parseDateRange extracts from/to from query params, defaulting to current month.
func parseDateRange(r *http.Request) (from, to time.Time) {
	now := time.Now().UTC()
	from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	to = now

	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t
		}
	}
	return from, to
}

func (h *ReportHandler) ProfitAndLoss(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	from, to := parseDateRange(r)
	pnl, err := h.repo.GetProfitAndLoss(r.Context(), userID, from, to)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"from":     pnl.From.Format("2006-01-02"),
		"to":       pnl.To.Format("2006-01-02"),
		"income":   pnl.Income.String(),
		"expenses": pnl.Expenses.String(),
		"net":      pnl.Net.String(),
	})
}

func (h *ReportHandler) BalanceSheet(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	asOf := time.Now().UTC()
	if v := r.URL.Query().Get("as_of"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			asOf = t
		}
	}

	bs, err := h.repo.GetBalanceSheet(r.Context(), userID, asOf)
	if err != nil {
		writeError(w, err)
		return
	}

	toBalanceList := func(items []app.AccountBalance) []map[string]string {
		out := make([]map[string]string, len(items))
		for i, a := range items {
			out[i] = map[string]string{
				"account_id": a.AccountID.String(),
				"name":       a.Name,
				"balance":    a.Balance.String(),
			}
		}
		return out
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"as_of":       bs.AsOf.Format("2006-01-02"),
		"assets":      toBalanceList(bs.Assets),
		"liabilities": toBalanceList(bs.Liabilities),
		"equity":      toBalanceList(bs.Equity),
		"net_worth":   bs.NetWorth.String(),
	})
}

func (h *ReportHandler) CashFlow(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	from, to := parseDateRange(r)
	periods, err := h.repo.GetCashFlow(r.Context(), userID, from, to)
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]map[string]string, len(periods))
	for i, p := range periods {
		out[i] = map[string]string{
			"month":   p.Month.Format("2006-01"),
			"inflow":  p.Inflow.String(),
			"outflow": p.Outflow.String(),
			"net":     p.Net.String(),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"periods": out})
}

func (h *ReportHandler) Categories(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	from, to := parseDateRange(r)
	cats, err := h.repo.GetCategoryBreakdown(r.Context(), userID, from, to)
	if err != nil {
		writeError(w, err)
		return
	}

	out := make([]map[string]string, len(cats))
	for i, c := range cats {
		out[i] = map[string]string{
			"category_id": c.CategoryID.String(),
			"name":        c.Name,
			"total":       c.Total.String(),
			"percentage":  c.Percentage.StringFixed(2),
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}
