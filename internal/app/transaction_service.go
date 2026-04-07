package app

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jonathanCaamano/fincontrol-back/internal/domain"
	"github.com/shopspring/decimal"
)

// TransactionService handles journal entry creation and voiding.
// RULE: entries are NEVER deleted or updated — only reversed.
type TransactionService struct {
	ledger LedgerRepository
	audit  AuditRepository
}

func NewTransactionService(ledger LedgerRepository, audit AuditRepository) *TransactionService {
	return &TransactionService{ledger: ledger, audit: audit}
}

func (s *TransactionService) Create(ctx context.Context, input CreateTransactionInput) (domain.JournalEntry, error) {
	// Validate double-entry invariant before touching the DB
	if len(input.Entries) < 2 {
		return domain.JournalEntry{}, fmt.Errorf("%w: minimum 2 entries required", domain.ErrInvalidInput)
	}

	var sum decimal.Decimal
	for _, e := range input.Entries {
		sum = sum.Add(e.Amount)
	}
	if !sum.IsZero() {
		return domain.JournalEntry{}, domain.ErrUnbalanced
	}

	journal, err := s.ledger.CreateJournalEntry(ctx, input)
	if err != nil {
		return domain.JournalEntry{}, err
	}

	_ = s.audit.Log(ctx, domain.AuditEntry{
		UserID:     input.UserID,
		Action:     "create_transaction",
		EntityType: "journal_entry",
		EntityID:   journal.ID,
	})

	return journal, nil
}

func (s *TransactionService) Void(ctx context.Context, id, userID uuid.UUID) (domain.JournalEntry, error) {
	reversal, err := s.ledger.VoidJournalEntry(ctx, id, userID)
	if err != nil {
		return domain.JournalEntry{}, err
	}

	_ = s.audit.Log(ctx, domain.AuditEntry{
		UserID:     userID,
		Action:     "void_transaction",
		EntityType: "journal_entry",
		EntityID:   id,
	})

	return reversal, nil
}

func (s *TransactionService) List(ctx context.Context, filters JournalFilters) ([]domain.JournalEntry, int, error) {
	return s.ledger.ListJournalEntries(ctx, filters)
}

func (s *TransactionService) Get(ctx context.Context, id, userID uuid.UUID) (domain.JournalEntry, error) {
	return s.ledger.GetJournalEntry(ctx, id, userID)
}
