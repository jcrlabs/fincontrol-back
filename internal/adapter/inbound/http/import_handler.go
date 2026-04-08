package http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/app"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/jonathanCaamano/fincontrol-back/internal/middleware"
	"github.com/shopspring/decimal"
)

type csvImportParser interface {
	SuggestMapping(r io.Reader) (domain.ColumnMapping, []domain.ImportRow, error)
	Parse(r io.Reader, mapping domain.ColumnMapping) ([]domain.ImportRow, error)
}

type ofxImportParser interface {
	Parse(r io.Reader) ([]domain.ImportRow, error)
}

type importConfirmer interface {
	Confirm(ctx context.Context, input app.ConfirmInput) (domain.ImportResult, error)
}

// ImportHandler handles /api/v1/import/* routes.
type ImportHandler struct {
	csv       csvImportParser
	ofx       ofxImportParser
	importSvc importConfirmer
}

func NewImportHandler(csv csvImportParser, ofx ofxImportParser, svc importConfirmer) *ImportHandler {
	return &ImportHandler{csv: csv, ofx: ofx, importSvc: svc}
}

// Preview parses the uploaded file and returns rows + suggested mapping.
func (h *ImportHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.UserIDFromContext(r.Context()); !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file too large or invalid multipart"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field required"})
		return
	}
	defer func() { _ = file.Close() }()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".csv":
		mapping, rows, err := h.csv.SuggestMapping(file)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"total_rows": len(rows), "suggested_mapping": mapping, "rows": toImportRowsJSON(rows),
		})
	case ".ofx", ".qfx":
		rows, err := h.ofx.Parse(file)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"total_rows": len(rows), "rows": toImportRowsJSON(rows)})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported file type, use .csv .ofx .qfx"})
	}
}

type confirmRowJSON struct {
	Date        string `json:"date"`
	Description string `json:"description"`
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	Hash        string `json:"hash"`
}

type confirmRequest struct {
	Rows            []confirmRowJSON      `json:"rows"`
	DebitAccountID  string                `json:"debit_account_id"`
	CreditAccountID string                `json:"credit_account_id,omitempty"` // optional: defaults to "Sin categorizar"
	CategoryID      *string               `json:"category_id,omitempty"`
	CSVData         string                `json:"csv_data,omitempty"`
	Mapping         *domain.ColumnMapping `json:"mapping,omitempty"`
}

// Confirm creates journal entries for the supplied rows.
func (h *ImportHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req confirmRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	debitID, err := uuid.Parse(req.DebitAccountID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid debit_account_id"})
		return
	}
	var creditIDPtr *uuid.UUID
	if req.CreditAccountID != "" {
		id, parseErr := uuid.Parse(req.CreditAccountID)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid credit_account_id"})
			return
		}
		creditIDPtr = &id
	}

	var rows []domain.ImportRow
	if req.CSVData != "" && req.Mapping != nil {
		rows, err = h.csv.Parse(bytes.NewBufferString(req.CSVData), *req.Mapping)
		if err != nil {
			writeError(w, err)
			return
		}
	} else {
		for _, rr := range req.Rows {
			row, parseErr := parseConfirmRow(rr)
			if parseErr != nil {
				continue
			}
			rows = append(rows, row)
		}
	}

	input := app.ConfirmInput{
		UserID: userID, Rows: rows,
		DebitAccountID: debitID, CreditAccountID: creditIDPtr,
	}
	if req.CategoryID != nil {
		if catID, err := uuid.Parse(*req.CategoryID); err == nil {
			input.CategoryID = &catID
		}
	}

	result, err := h.importSvc.Confirm(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"imported": result.Imported, "duplicates": result.Duplicates, "errors": result.Errors,
	})
}

func parseConfirmRow(rr confirmRowJSON) (domain.ImportRow, error) {
	date, err := time.Parse("2006-01-02", rr.Date)
	if err != nil {
		return domain.ImportRow{}, err
	}
	amount, err := decimal.NewFromString(rr.Amount)
	if err != nil {
		return domain.ImportRow{}, err
	}
	cur := rr.Currency
	if cur == "" {
		cur = "EUR"
	}
	return domain.ImportRow{
		Date: date, Description: rr.Description,
		Amount: amount, Currency: cur, Hash: rr.Hash,
	}, nil
}

func toImportRowsJSON(rows []domain.ImportRow) []map[string]string {
	out := make([]map[string]string, len(rows))
	for i, r := range rows {
		out[i] = map[string]string{
			"date": r.Date.Format("2006-01-02"), "description": r.Description,
			"amount": r.Amount.String(), "currency": r.Currency, "hash": r.Hash,
		}
	}
	return out
}
