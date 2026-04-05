package http

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/jonathanCaamano/fincontrol-back/internal/middleware"
)

type scheduledService interface {
	Create(ctx context.Context, input app.CreateScheduledInput) (domain.ScheduledTransaction, error)
	List(ctx context.Context, userID uuid.UUID) ([]domain.ScheduledTransaction, error)
	Update(ctx context.Context, input app.UpdateScheduledInput) (domain.ScheduledTransaction, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
}

// ScheduledHandler handles /api/v1/scheduled routes.
type ScheduledHandler struct {
	svc scheduledService
}

func NewScheduledHandler(svc scheduledService) *ScheduledHandler {
	return &ScheduledHandler{svc: svc}
}

type scheduledResponse struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Frequency   string `json:"frequency"`
	NextRun     string `json:"next_run"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	Entries     []struct {
		AccountID string `json:"account_id"`
		Amount    string `json:"amount"`
		Currency  string `json:"currency"`
	} `json:"entries"`
}

func toScheduledResponse(s domain.ScheduledTransaction) scheduledResponse {
	resp := scheduledResponse{
		ID:          s.ID.String(),
		Description: s.Description,
		Frequency:   string(s.Frequency),
		NextRun:     s.NextRun.Format("2006-01-02T15:04:05Z07:00"),
		IsActive:    s.IsActive,
		CreatedAt:   s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	for _, e := range s.TemplateEntries {
		resp.Entries = append(resp.Entries, struct {
			AccountID string `json:"account_id"`
			Amount    string `json:"amount"`
			Currency  string `json:"currency"`
		}{
			AccountID: e.AccountID.String(),
			Amount:    e.Amount,
			Currency:  e.Currency,
		})
	}
	return resp
}

func (h *ScheduledHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	items, err := h.svc.List(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]scheduledResponse, len(items))
	for i, s := range items {
		resp[i] = toScheduledResponse(s)
	}
	writeJSON(w, http.StatusOK, resp)
}

type scheduledEntryInput struct {
	AccountID string `json:"account_id"`
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
}

type createScheduledRequest struct {
	Description string                `json:"description"`
	Frequency   string                `json:"frequency"`
	NextRun     string                `json:"next_run"`
	Entries     []scheduledEntryInput `json:"entries"`
}

func (h *ScheduledHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createScheduledRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	nextRun, err := time.Parse("2006-01-02T15:04:05Z07:00", req.NextRun)
	if err != nil {
		// Try simple date format
		nextRun, err = time.Parse("2006-01-02", req.NextRun)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid next_run format"})
			return
		}
	}

	entries := make([]domain.ScheduledEntry, len(req.Entries))
	for i, e := range req.Entries {
		accountID, err := uuid.Parse(e.AccountID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id in entry"})
			return
		}
		currency := e.Currency
		if currency == "" {
			currency = "EUR"
		}
		entries[i] = domain.ScheduledEntry{
			AccountID: accountID,
			Amount:    e.Amount,
			Currency:  currency,
		}
	}

	s, err := h.svc.Create(r.Context(), app.CreateScheduledInput{
		UserID:      userID,
		Description: req.Description,
		Frequency:   domain.Frequency(req.Frequency),
		NextRun:     nextRun,
		Entries:     entries,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toScheduledResponse(s))
}

type updateScheduledRequest struct {
	Frequency string `json:"frequency,omitempty"`
	NextRun   string `json:"next_run,omitempty"`
	IsActive  bool   `json:"is_active"`
}

func (h *ScheduledHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scheduled id"})
		return
	}

	var req updateScheduledRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	input := app.UpdateScheduledInput{
		ID:       id,
		UserID:   userID,
		IsActive: req.IsActive,
	}
	if req.Frequency != "" {
		input.Frequency = domain.Frequency(req.Frequency)
	}
	if req.NextRun != "" {
		if t, err := time.Parse("2006-01-02", req.NextRun); err == nil {
			input.NextRun = t
		}
	}

	s, err := h.svc.Update(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toScheduledResponse(s))
}

func (h *ScheduledHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid scheduled id"})
		return
	}

	if err := h.svc.Delete(r.Context(), id, userID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}
