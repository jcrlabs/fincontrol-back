package domain

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Entry is one line of a double-entry journal.
// Positive amount = debit, negative amount = credit.
type Entry struct {
	ID             uuid.UUID
	JournalEntryID uuid.UUID
	AccountID      uuid.UUID
	Amount         decimal.Decimal // positive = debit, negative = credit
	Currency       string
	CreatedAt      time.Time
}
