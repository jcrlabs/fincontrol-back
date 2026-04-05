package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// JournalEntry is the header of a double-entry transaction.
// INVARIANT: sum(entries.amount) == 0 ALWAYS.
// RULE: journal entries are NEVER deleted or updated. Void with a reverse entry.
type JournalEntry struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Description     string
	Date            time.Time
	CategoryID      *uuid.UUID
	IsReversal      bool
	ReversedEntryID *uuid.UUID
	Entries         []Entry
	CreatedAt       time.Time
}

// Validate checks the double-entry invariant.
func (j JournalEntry) Validate() error {
	if len(j.Entries) < 2 {
		return ErrInvalidInput
	}
	var sum decimal.Decimal
	for _, e := range j.Entries {
		sum = sum.Add(e.Amount)
	}
	if !sum.IsZero() {
		return ErrUnbalanced
	}
	return nil
}
