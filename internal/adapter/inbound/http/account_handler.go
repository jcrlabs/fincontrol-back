package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/jonathanCaamano/fincontrol-back/internal/middleware"
	"github.com/shopspring/decimal"
)

type accountService interface {
	Create(ctx context.Context, input app.CreateAccountInput) (domain.Account, error)
	GetByID(ctx context.Context, id, userID uuid.UUID) (domain.Account, error)
	List(ctx context.Context, userID uuid.UUID) ([]domain.Account, error)
	Update(ctx context.Context, input app.UpdateAccountInput) (domain.Account, error)
}

type ledgerRepo interface {
	ListEntriesByAccount(ctx context.Context, accountID, userID uuid.UUID, page, pageSize int) ([]domain.Entry, int, error)
}

// AccountHandler handles /api/v1/accounts routes.
type AccountHandler struct {
	svc    accountService
	ledger ledgerRepo
}

func NewAccountHandler(svc accountService, ledger ledgerRepo) *AccountHandler {
	return &AccountHandler{svc: svc, ledger: ledger}
}

// accountResponse is the JSON representation of an account.
type accountResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Currency  string `json:"currency"`
	IsActive  bool   `json:"is_active"`
	Balance   string `json:"balance"`
	CreatedAt string `json:"created_at"`
}

func toAccountResponse(a domain.Account) accountResponse {
	return accountResponse{
		ID:        a.ID.String(),
		Name:      a.Name,
		Type:      string(a.Type),
		Currency:  a.Currency,
		IsActive:  a.IsActive,
		Balance:   a.Balance.String(),
		CreatedAt: a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *AccountHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	accounts, err := h.svc.List(r.Context(), userID)
	if err != nil {
		writeError(w, err)
		return
	}

	resp := make([]accountResponse, len(accounts))
	for i, a := range accounts {
		resp[i] = toAccountResponse(a)
	}
	writeJSON(w, http.StatusOK, resp)
}

type createAccountRequest struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Currency       string `json:"currency"`
	InitialBalance string `json:"initial_balance"`
}

func (h *AccountHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	initialBalance := decimal.Zero
	if req.InitialBalance != "" {
		var err error
		initialBalance, err = decimal.NewFromString(req.InitialBalance)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid initial_balance"})
			return
		}
	}

	account, err := h.svc.Create(r.Context(), app.CreateAccountInput{
		UserID:         userID,
		Name:           req.Name,
		Type:           domain.AccountType(req.Type),
		Currency:       req.Currency,
		InitialBalance: initialBalance,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAccountResponse(account))
}

func (h *AccountHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}

	account, err := h.svc.GetByID(r.Context(), id, userID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAccountResponse(account))
}

type updateAccountRequest struct {
	Name string `json:"name"`
}

func (h *AccountHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}

	var req updateAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	account, err := h.svc.Update(r.Context(), app.UpdateAccountInput{
		ID:     id,
		UserID: userID,
		Name:   req.Name,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAccountResponse(account))
}

func (h *AccountHandler) ListEntries(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	accountID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid account id"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	entries, total, err := h.ledger.ListEntriesByAccount(r.Context(), accountID, userID, page, pageSize)
	if err != nil {
		writeError(w, err)
		return
	}

	type entryResponse struct {
		ID             string `json:"id"`
		JournalEntryID string `json:"journal_entry_id"`
		AccountID      string `json:"account_id"`
		Amount         string `json:"amount"`
		Currency       string `json:"currency"`
		CreatedAt      string `json:"created_at"`
	}

	resp := make([]entryResponse, len(entries))
	for i, e := range entries {
		resp[i] = entryResponse{
			ID:             e.ID.String(),
			JournalEntryID: e.JournalEntryID.String(),
			AccountID:      e.AccountID.String(),
			Amount:         e.Amount.String(),
			Currency:       e.Currency,
			CreatedAt:      e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":    resp,
		"total":    total,
		"page":     page,
		"per_page": pageSize,
	})
}
