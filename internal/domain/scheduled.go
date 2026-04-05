package domain

import (
	"time"

	"github.com/google/uuid"
)

// Frequency defines how often a scheduled transaction runs.
type Frequency string

const (
	FrequencyDaily   Frequency = "daily"
	FrequencyWeekly  Frequency = "weekly"
	FrequencyMonthly Frequency = "monthly"
)

// ScheduledTransaction is a recurring transaction template evaluated by the scheduler.
type ScheduledTransaction struct {
	ID                  uuid.UUID
	UserID              uuid.UUID
	Description         string
	Frequency           Frequency
	NextRun             time.Time
	IsActive            bool
	TemplateEntries     []ScheduledEntry
	CreatedAt           time.Time
}

// ScheduledEntry is a template entry for a scheduled transaction.
type ScheduledEntry struct {
	AccountID uuid.UUID
	Amount    string // stored as string to preserve precision
	Currency  string
}
