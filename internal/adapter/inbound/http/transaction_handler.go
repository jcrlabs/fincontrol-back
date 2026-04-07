package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/jonathanCaamano/fincontrol-back/internal/middleware"
	"github.com/shopspring/decimal"
)

type transactionService interface {
	Create(ctx context.Context, input app.CreateTransactionInput) (domain.JournalEntry, error)
	List(ctx context.Context, filters app.JournalFilters) ([]domain.JournalEntry, int, error)
	Get(ctx context.Context, id, userID uuid.UUID) (domain.JournalEntry, error)
	Void(ctx context.Context, id, userID uuid.UUID) (domain.JournalEntry, error)
}

// TransactionHandler handles /api/v1/transactions routes.
type TransactionHandler struct {
	svc transactionService
}

func NewTransactionHandler(svc transactionService) *TransactionHandler {
	return &TransactionHandler{svc: svc}
}

// --- Response types ---

type entryResponse struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
}

type journalEntryResponse struct {
	ID              string          `json:"id"`
	Description     string          `json:"description"`
	Date            string          `json:"date"`
	CategoryID      *string         `json:"category_id,omitempty"`
	IsReversal      bool            `json:"is_reversal"`
	ReversedEntryID *string         `json:"reversed_entry_id,omitempty"`
	Entries         []entryResponse `json:"entries"`
	CreatedAt       string          `json:"created_at"`
}

func toJournalResponse(j domain.JournalEntry) journalEntryResponse {
	resp := journalEntryResponse{
		ID:          j.ID.String(),
		Description: j.Description,
		Date:        j.Date.Format("2006-01-02"),
		IsReversal:  j.IsReversal,
		CreatedAt:   j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if j.CategoryID != nil {
		s := j.CategoryID.String()
		resp.CategoryID = &s
	}
	if j.ReversedEntryID != nil {
		s := j.ReversedEntryID.String()
		resp.ReversedEntryID = &s
	}
	resp.Entries = make([]entryResponse, len(j.Entries))
	for i, e := range j.Entries {
		resp.Entries[i] = entryResponse{
			ID:        e.ID.String(),
			AccountID: e.AccountID.String(),
			Amount:    e.Amount.String(),
			Currency:  e.Currency,
		}
	}
	return resp
}

// --- Handlers ---

func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	filters := app.JournalFilters{
		UserID: userID,
	}

	if v := r.URL.Query().Get("date_from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			filters.DateFrom = &t
		}
	}
	if v := r.URL.Query().Get("date_to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err == nil {
			filters.DateTo = &t
		}
	}
	if v := r.URL.Query().Get("account_id"); v != "" {
		id, err := uuid.Parse(v)
		if err == nil {
			filters.AccountID = &id
		}
	}
	if v := r.URL.Query().Get("category_id"); v != "" {
		id, err := uuid.Parse(v)
		if err == nil {
			filters.CategoryID = &id
		}
	}
	filters.Page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	filters.PageSize, _ = strconv.Atoi(r.URL.Query().Get("page_size"))

	journals, total, err := h.svc.List(r.Context(), filters)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]journalEntryResponse, len(journals))
	for i, j := range journals {
		resp[i] = toJournalResponse(j)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resp, "total": total})
}

type entryInput struct {
	AccountID string `json:"account_id"`
	Amount    string `json:"amount"`
	Currency  string `json:"currency"`
}

type createTransactionRequest struct {
	Date        string       `json:"date"`
	Description string       `json:"description"`
	CategoryID  string       `json:"category_id,omitempty"`
	Entries     []entryInput `json:"entries"`
}

func (h *TransactionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createTransactionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date format, use YYYY-MM-DD"})
		return
	}

	if len(req.Entries) < 2 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "minimum 2 entries required"})
		return
	}

	entries := make([]app.EntryInput, len(req.Entries))
	for i, e := range req.Entries {
		accountID, err := uuid.Parse(e.AccountID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account_id in entry"})
			return
		}
		amount, err := decimal.NewFromString(e.Amount)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid amount in entry"})
			return
		}
		currency := e.Currency
		if currency == "" {
			currency = "EUR"
		}
		entries[i] = app.EntryInput{
			AccountID: accountID,
			Amount:    amount,
			Currency:  currency,
		}
	}

	input := app.CreateTransactionInput{
		UserID:      userID,
		Description: req.Description,
		Date:        date,
		Entries:     entries,
	}
	if req.CategoryID != "" {
		catID, err := uuid.Parse(req.CategoryID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid category_id"})
			return
		}
		input.CategoryID = &catID
	}

	journal, err := h.svc.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toJournalResponse(journal))
}

func (h *TransactionHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid transaction id"})
		return
	}

	journal, err := h.svc.Get(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toJournalResponse(journal))
}

func (h *TransactionHandler) Void(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid transaction id"})
		return
	}

	reversal, err := h.svc.Void(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toJournalResponse(reversal))
}
