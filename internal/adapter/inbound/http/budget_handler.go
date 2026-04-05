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

type budgetService interface {
	Create(ctx context.Context, input app.CreateBudgetInput) (domain.Budget, error)
	ListWithProgress(ctx context.Context, userID uuid.UUID, month time.Time) ([]domain.BudgetProgress, error)
	Update(ctx context.Context, input app.UpdateBudgetInput) (domain.Budget, error)
}

// BudgetHandler handles /api/v1/budgets routes.
type BudgetHandler struct {
	svc budgetService
}

func NewBudgetHandler(svc budgetService) *BudgetHandler {
	return &BudgetHandler{svc: svc}
}

type budgetResponse struct {
	ID                string `json:"id"`
	CategoryID        string `json:"category_id"`
	Month             string `json:"month"`
	Amount            string `json:"amount"`
	AlertThresholdPct int    `json:"alert_threshold_pct"`
}

type budgetProgressResponse struct {
	budgetResponse
	Spent      string `json:"spent"`
	Percentage string `json:"percentage"`
	IsAlert    bool   `json:"is_alert"`
	IsExceeded bool   `json:"is_exceeded"`
}

func toBudgetResponse(b domain.Budget) budgetResponse {
	return budgetResponse{
		ID:                b.ID.String(),
		CategoryID:        b.CategoryID.String(),
		Month:             b.Month.Format("2006-01"),
		Amount:            b.Amount.String(),
		AlertThresholdPct: b.AlertThresholdPct,
	}
}

func toBudgetProgressResponse(bp domain.BudgetProgress) budgetProgressResponse {
	return budgetProgressResponse{
		budgetResponse: toBudgetResponse(bp.Budget),
		Spent:          bp.Spent.String(),
		Percentage:     bp.Percentage.StringFixed(2),
		IsAlert:        bp.IsAlert,
		IsExceeded:     bp.IsExceeded,
	}
}

func (h *BudgetHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	month := time.Now().UTC()
	if v := r.URL.Query().Get("month"); v != "" {
		if t, err := time.Parse("2006-01", v); err == nil {
			month = t
		}
	}

	progress, err := h.svc.ListWithProgress(r.Context(), userID, month)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]budgetProgressResponse, len(progress))
	for i, bp := range progress {
		resp[i] = toBudgetProgressResponse(bp)
	}
	writeJSON(w, http.StatusOK, resp)
}

type createBudgetRequest struct {
	CategoryID        string `json:"category_id"`
	Month             string `json:"month"`
	Amount            string `json:"amount"`
	AlertThresholdPct int    `json:"alert_threshold_pct,omitempty"`
}

func (h *BudgetHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createBudgetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category_id"})
		return
	}

	month, err := time.Parse("2006-01", req.Month)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid month format, use YYYY-MM"})
		return
	}

	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount"})
		return
	}

	budget, err := h.svc.Create(r.Context(), app.CreateBudgetInput{
		UserID:            userID,
		CategoryID:        catID,
		Month:             month,
		Amount:            amount,
		AlertThresholdPct: req.AlertThresholdPct,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toBudgetResponse(budget))
}

type updateBudgetRequest struct {
	Amount            string `json:"amount,omitempty"`
	AlertThresholdPct int    `json:"alert_threshold_pct,omitempty"`
}

func (h *BudgetHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid budget id"})
		return
	}

	var req updateBudgetRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	input := app.UpdateBudgetInput{
		ID:                id,
		UserID:            userID,
		AlertThresholdPct: req.AlertThresholdPct,
	}
	if req.Amount != "" {
		amount, err := decimal.NewFromString(req.Amount)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount"})
			return
		}
		input.Amount = amount
	}

	budget, err := h.svc.Update(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toBudgetResponse(budget))
}
