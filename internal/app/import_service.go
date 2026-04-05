package app

import (
	"context"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
)

// ImportParser parses a file into ImportRows using a column mapping.
type ImportParser interface {
	Parse(r io.Reader, mapping domain.ColumnMapping) ([]domain.ImportRow, error)
	SuggestMapping(r io.Reader) (domain.ColumnMapping, []domain.ImportRow, error)
}

// ImportDedupRepository checks and records imported row hashes.
type ImportDedupRepository interface {
	IsDuplicate(ctx context.Context, userID uuid.UUID, hash string) (bool, error)
	RecordImport(ctx context.Context, userID uuid.UUID, hash string, journalID uuid.UUID) error
}

// ImportService handles CSV/OFX file import with dedup.
type ImportService struct {
	dedup  ImportDedupRepository
	ledger LedgerRepository
	audit  AuditRepository
}

func NewImportService(dedup ImportDedupRepository, ledger LedgerRepository, audit AuditRepository) *ImportService {
	return &ImportService{dedup: dedup, ledger: ledger, audit: audit}
}

// Preview parses the file and returns rows + suggested mapping without importing.
func Preview(parser ImportParser, r io.Reader) (domain.ImportPreview, error) {
	mapping, rows, err := parser.SuggestMapping(r)
	if err != nil {
		return domain.ImportPreview{}, fmt.Errorf("suggest mapping: %w", err)
	}
	return domain.ImportPreview{
		Rows:             rows,
		SuggestedMapping: mapping,
		TotalRows:        len(rows),
	}, nil
}

// ConfirmInput holds everything needed to execute an import.
type ConfirmInput struct {
	UserID          uuid.UUID
	Rows            []domain.ImportRow
	DebitAccountID  uuid.UUID // account to debit (expense account)
	CreditAccountID uuid.UUID // account to credit (payment account)
	CategoryID      *uuid.UUID
}

// Confirm creates journal entries for the given rows, skipping duplicates.
func (s *ImportService) Confirm(ctx context.Context, input ConfirmInput) (domain.ImportResult, error) {
	result := domain.ImportResult{}

	for _, row := range input.Rows {
		isDup, err := s.dedup.IsDuplicate(ctx, input.UserID, row.Hash)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("check dedup for %s: %v", row.Description, err))
			continue
		}
		if isDup {
			result.Duplicates++
			continue
		}

		journal, err := s.ledger.CreateJournalEntry(ctx, CreateTransactionInput{
			UserID:      input.UserID,
			Description: row.Description,
			Date:        row.Date,
			CategoryID:  input.CategoryID,
			Entries: []EntryInput{
				{AccountID: input.DebitAccountID, Amount: row.Amount, Currency: row.Currency},
				{AccountID: input.CreditAccountID, Amount: row.Amount.Neg(), Currency: row.Currency},
			},
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("create entry for %s: %v", row.Description, err))
			continue
		}

		if err := s.dedup.RecordImport(ctx, input.UserID, row.Hash, journal.ID); err != nil {
			// Non-fatal: entry created but dedup record failed
			result.Errors = append(result.Errors, fmt.Sprintf("record dedup for %s: %v", row.Description, err))
		}

		s.audit.Log(ctx, domain.AuditEntry{ //nolint:gosec
			UserID:     input.UserID,
			Action:     "import_transaction",
			EntityType: "journal_entry",
			EntityID:   journal.ID,
		})

		result.Imported++
	}

	return result, nil
}
